# cost-check: AWS MCP の run_script に、このファイルの中身をそのまま渡す (書き換えない)
#   - CLI (scripts/cli/check.sh) とは別の経路で、時間課金のリソースと CloudWatch アラームを読み取りで確かめる
#   - 読み取りの API (Describe* / List* / Get*) だけを呼ぶ
#   - 並び順を固定し、時刻は JST で返す。同じ状態なら、時刻を除いて同じ結果になる
#   - API が失敗したら、その項目は {"error": "..."} になる (空と区別するため)
#   - 認証の不備 (認証情報を取れない、アカウントを確かめられない) なら、ほかを調べずに {"authError": "..."} を返す

import asyncio
import datetime
from typing import Any

# call_boto3 は、run_script の実行環境が用意する関数 (このファイルの中では定義しない)
call_boto3 = globals()["call_boto3"]

REGION = "ap-northeast-1"
JST = datetime.timezone(datetime.timedelta(hours=9))


def now():
    return datetime.datetime.now(JST).strftime("%Y-%m-%d %H:%M:%S")


async def call(service: str, operation: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
    try:
        return await call_boto3(
            service_name=service, operation_name=operation, region_name=REGION, params=params or {}
        )
    except Exception as e:  # noqa: BLE001
        return {"error": f"{type(e).__name__}: {e}"}


def pick(resp: dict[str, Any], fn: Any) -> Any:
    if "error" in resp:
        return resp
    return fn(resp)


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

    clusters, lbs, tgs, endpoints, nats, instances, enis, alarms = await asyncio.gather(
        call("ecs", "DescribeClusters", {"clusters": ["prd-amber"]}),
        call("elbv2", "DescribeLoadBalancers"),
        call("elbv2", "DescribeTargetGroups"),
        call("ec2", "DescribeVpcEndpoints", {"Filters": vpc_filter}),
        call("ec2", "DescribeNatGateways", {"Filter": vpc_filter}),
        call("ec2", "DescribeInstances", {"Filters": vpc_filter}),
        call("ec2", "DescribeNetworkInterfaces", {"Filters": vpc_filter}),
        call("cloudwatch", "DescribeAlarms"),
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
        "alarms": pick(alarms, lambda r: sorted(
            [a["AlarmName"] for a in r.get("MetricAlarms", []) + r.get("CompositeAlarms", [])
                if "prd-amber" in a["AlarmName"]])),
        "finishedAtJst": now(),
    }


# run_script の実行環境は、トップレベルの await を受け付ける (エディターの解析は、この行だけ止める)
result = await main()  # pyright: ignore
# run_script は、最後の式の値を結果として返す
result  # pyright: ignore[reportUnusedExpression]
