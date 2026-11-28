# cost-check: AWS MCP の run_script に、このファイルの中身をそのまま渡す (書き換えない)
#   - CLI (scripts/cli/check.sh) とは別の経路で、時間課金のリソース (ECS、ELB、VPC エンドポイント、NAT、EC2、ENI、EIP、EBS、
#     オートスケーリング) と CloudWatch アラームと、tfstate のバケットの古い版を読み取りで確かめる
#   - run_script では globals 関数などが禁止されている。実行環境が用意する call_boto3 は、そのまま参照する
#   - 読み取りの API (Describe* / List* / Get*) だけを呼ぶ
#   - 並び順を固定し、時刻は JST で返す。同じ状態なら、時刻を除いて同じ結果になる
#   - API が失敗したら、その項目は {"error": "..."} になる (空と区別するため)
#   - 認証の不備 (認証情報を取れない、アカウントを確かめられない) なら、ほかを調べずに {"authError": "..."} を返す

import asyncio
import datetime
from typing import Any

REGION = "ap-northeast-1"
JST = datetime.timezone(datetime.timedelta(hours=9))
DAY = 86400

# tfstate のバケットのライフサイクル (infra/tfstate/lifecycle.json) の値と、消える条件を満たしてから待つ日数
NONCURRENT_DAYS = 1
NEWER_NONCURRENT_VERSIONS = 10
GRACE_DAYS = 3


def now():
    return datetime.datetime.now(JST).strftime("%Y-%m-%d %H:%M:%S")


async def call(service: str, operation: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
    try:
        # call_boto3 は run_script の実行環境が用意する関数。globals 関数は run_script で禁止されているため、そのまま参照する
        return await call_boto3(  # pyright: ignore[reportUndefinedVariable]
            service_name=service, operation_name=operation, region_name=REGION, params=params or {}
        )
    except Exception as e:  # noqa: BLE001
        return {"error": f"{type(e).__name__}: {e}"}


def pick(resp: dict[str, Any], fn: Any) -> Any:
    if "error" in resp:
        return resp
    return fn(resp)


def epoch(last_modified: str) -> int:
    return int(datetime.datetime.fromisoformat(last_modified).timestamp())


def jst(t: int) -> str:
    return datetime.datetime.fromtimestamp(t, JST).strftime("%Y-%m-%d %H:%M:%S")


def next_utc_midnight(t: int) -> int:
    return (t // DAY + 1) * DAY


def tfstate_key(entries: list[dict[str, Any]], now_epoch: int) -> dict[str, Any]:
    # 今の版を先頭に、古い版を新しい順に並べる。古い版 n 番目 (all_[n]) は、all_[n - 1] が作られた時刻に古い版になった
    current = [e for e in entries if e["latest"]]
    noncurrent = sorted([e for e in entries if not e["latest"]], key=lambda e: -e["t"])
    all_ = current + noncurrent
    excess = []
    for n in range(NEWER_NONCURRENT_VERSIONS + 1, len(all_)):
        # 日数の条件 (古い版になってからの日数を、次の UTC 0 時に切り上げる) と、数の条件 (新しい古い版がそろった時刻) の遅いほう
        eligible = max(
            next_utc_midnight(all_[n - 1]["t"] + NONCURRENT_DAYS * DAY),
            all_[n - NEWER_NONCURRENT_VERSIONS - 1]["t"],
        )
        excess.append({
            "noncurrentSinceJst": jst(all_[n - 1]["t"]),
            "eligibleAtJst": jst(eligible),
            "deadlineJst": jst(eligible + GRACE_DAYS * DAY),
            "overdue": now_epoch > eligible + GRACE_DAYS * DAY,
        })
    overdue = len([x for x in excess if x["overdue"]])
    verdict = "OK"
    if excess:
        verdict = "OK_PENDING"
    if overdue > 0:
        verdict = "NG"
    return {
        "key": all_[0]["key"],
        "noncurrent": len(all_) - 1,
        "excess": len(excess),
        "overdue": overdue,
        "verdict": verdict,
        "excessVersions": [{k: v for k, v in x.items() if k != "overdue"} for x in excess],
    }


def tfstate_versions(resp: dict[str, Any]) -> list[dict[str, Any]]:
    # scripts/cli/checks/storage.sh の tfstate の確認と、同じ値と判定を返す (判定の決め方は storage.sh のコメントを参照)
    # DOC: https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html
    # DOC: https://repost.aws/knowledge-center/s3-lifecycle-rule-delay
    now_epoch = int(datetime.datetime.now(datetime.timezone.utc).timestamp())
    entries = [{"key": x["Key"], "latest": x["IsLatest"], "t": epoch(x["LastModified"])}
               for x in resp.get("Versions", []) + resp.get("DeleteMarkers", [])]
    keys = sorted({e["key"] for e in entries})
    return [tfstate_key([e for e in entries if e["key"] == k], now_epoch) for k in keys]


async def main():
    started_at = now()

    identity, vpcs = await asyncio.gather(
        call("sts", "GetCallerIdentity"),
        call("ec2", "DescribeVpcs", {"Filters": [{"Name": "tag:project", "Values": ["amber"]}]}),
    )
    if "error" in identity:
        return {"authError": identity["error"], "checkedAtJst": started_at}

    # amber のアカウントか (tfstate のバケットの持ち主か) を確かめる。アカウント ID はリポジトリに書かない
    owner = await call("s3", "HeadBucket", {"Bucket": "prd-amber-tfstate", "ExpectedBucketOwner": identity["Account"]})
    if "error" in owner:
        return {"authError": f"not the amber account: {owner['error']}", "checkedAtJst": started_at}

    vpc_id = (vpcs.get("Vpcs") or [{}])[0].get("VpcId") if "error" not in vpcs else None
    vpc_filter = [{"Name": "vpc-id", "Values": [vpc_id or "none"]}]

    amber_tag = [{"Name": "tag:project", "Values": ["amber"]}]
    clusters, lbs, tgs, endpoints, nats, instances, enis, eips, volumes, scalable, alarms, versions = await asyncio.gather(
        call("ecs", "DescribeClusters", {"clusters": ["prd-amber"]}),
        call("elbv2", "DescribeLoadBalancers"),
        call("elbv2", "DescribeTargetGroups"),
        call("ec2", "DescribeVpcEndpoints", {"Filters": vpc_filter}),
        call("ec2", "DescribeNatGateways", {"Filter": vpc_filter}),
        call("ec2", "DescribeInstances", {"Filters": vpc_filter}),
        call("ec2", "DescribeNetworkInterfaces", {"Filters": vpc_filter}),
        call("ec2", "DescribeAddresses", {"Filters": amber_tag}),
        call("ec2", "DescribeVolumes", {"Filters": amber_tag}),
        call("application-autoscaling", "DescribeScalableTargets", {"ServiceNamespace": "ecs"}),
        call("cloudwatch", "DescribeAlarms"),
        call("s3", "ListObjectVersions", {"Bucket": "prd-amber-tfstate"}),
    )

    return {
        "startedAtJst": started_at,
        "account": pick(identity, lambda r: r.get("Account")),
        "region": REGION,
        "amberVpcFound": vpc_id is not None,
        "ecsClusters": pick(clusters, lambda r: sorted(
            [{"name": c["clusterName"], "status": c["status"], "runningTasks": c["runningTasksCount"],
                "services": c["activeServicesCount"]} for c in r.get("clusters", [])],
            key=lambda x: x["name"])),
        "loadBalancers": pick(lbs, lambda r: sorted(
            [x["LoadBalancerName"] for x in r.get("LoadBalancers", []) if "prd-amber" in x["LoadBalancerName"]])),
        "targetGroups": pick(tgs, lambda r: sorted(
            [x["TargetGroupName"] for x in r.get("TargetGroups", []) if "prd-amber" in x["TargetGroupName"]])),
        "vpcEndpoints": pick(endpoints, lambda r: sorted(
            [{"type": x["VpcEndpointType"], "service": x["ServiceName"], "state": x["State"]}
                for x in r.get("VpcEndpoints", []) if x["State"] != "deleted"],
            key=lambda x: x["service"])),
        "natGateways": pick(nats, lambda r: sorted(
            [x["NatGatewayId"] for x in r.get("NatGateways", []) if x["State"] != "deleted"])),
        "instances": pick(instances, lambda r: sorted(
            [i["InstanceId"] for res in r.get("Reservations", []) for i in res["Instances"]
                if i["State"]["Name"] != "terminated"])),
        "networkInterfaces": pick(enis, lambda r: sorted(
            [{"type": x.get("InterfaceType"), "description": x.get("Description"),
                "publicIp": x.get("Association", {}).get("PublicIp")} for x in r.get("NetworkInterfaces", [])],
            key=lambda x: x["description"] or "")),
        "elasticIps": pick(eips, lambda r: sorted(
            [x["PublicIp"] for x in r.get("Addresses", [])])),
        "ebsVolumes": pick(volumes, lambda r: sorted(
            [x["VolumeId"] for x in r.get("Volumes", [])])),
        "scalableTargets": pick(scalable, lambda r: sorted(
            [x["ResourceId"] for x in r.get("ScalableTargets", []) if "prd-amber" in x["ResourceId"]])),
        "alarms": pick(alarms, lambda r: sorted(
            [a["AlarmName"] for a in r.get("MetricAlarms", []) + r.get("CompositeAlarms", [])
                if "prd-amber" in a["AlarmName"]])),
        "tfstateVersions": pick(versions, tfstate_versions),
        "finishedAtJst": now(),
    }


# run_script の実行環境は、トップレベルの await を受け付ける (エディターの解析は、この行だけ止める)
result = await main()  # pyright: ignore
# run_script は、最後の式の値を結果として返す
result  # pyright: ignore[reportUnusedExpression]
