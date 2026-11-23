#!/usr/bin/env bash
# 時間課金のリソース: ECS、ELB、VPC エンドポイント、NAT、EIP、EC2、EBS、ENI、オートスケーリング
#   - amber のものに絞る (名前に prd-amber を含む、amber の VPC にある、タグ project=amber)
#   - ECS のクラスターは、止めた直後は INACTIVE、しばらくすると一覧から消える。どちらも「残っていない」

# JMESPath のリテラル (`deleted` など) を展開させないため、--query はわざとシングルクォートで囲む
# shellcheck disable=SC2016
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

vpc_id=$(amber_vpc_id)

section "ECS: ACTIVE なクラスター (prd-amber のものがない)"
query '[.clusters[] | select(.clusterName | contains("prd-amber")) | {name: .clusterName, status, runningTasks: .runningTasksCount, services: .activeServicesCount}] | sort_by(.name)' \
    aws ecs describe-clusters --clusters prd-amber
query '[.clusterArns[] | select(contains("prd-amber"))] | sort' \
    aws ecs list-clusters

section "ELB: ロードバランサーとターゲットグループ (prd-amber のものがない)"
query '[.LoadBalancers[] | select(.LoadBalancerName | contains("prd-amber")) | {name: .LoadBalancerName, state: .State.Code}] | sort_by(.name)' \
    aws elbv2 describe-load-balancers
query '[.TargetGroups[] | select(.TargetGroupName | contains("prd-amber")) | .TargetGroupName] | sort' \
    aws elbv2 describe-target-groups

section "VPC エンドポイント: amber の VPC (削除済みを除く。Gateway の S3 だけなら無料)"
query '[.VpcEndpoints[] | select(.State != "deleted") | {type: .VpcEndpointType, service: .ServiceName, state: .State}] | sort_by(.service)' \
    aws ec2 describe-vpc-endpoints --filters "Name=vpc-id,Values=${vpc_id}"

section "NAT ゲートウェイ、EC2、ENI: amber の VPC (どれもない)"
query '[.NatGateways[] | select(.State != "deleted") | {id: .NatGatewayId, state: .State}] | sort_by(.id)' \
    aws ec2 describe-nat-gateways --filter "Name=vpc-id,Values=${vpc_id}"
query '[.Reservations[].Instances[] | select(.State.Name != "terminated") | .InstanceId] | sort' \
    aws ec2 describe-instances --filters "Name=vpc-id,Values=${vpc_id}"
query '[.NetworkInterfaces[] | {type: .InterfaceType, description: .Description, publicIp: .Association.PublicIp}] | sort_by(.description)' \
    aws ec2 describe-network-interfaces --filters "Name=vpc-id,Values=${vpc_id}"

section "EIP、EBS: タグ project=amber (どれもない)"
query '[.Addresses[] | {ip: .PublicIp, association: .AssociationId}] | sort_by(.ip)' \
    aws ec2 describe-addresses --filters Name=tag:project,Values=amber
query '[.Volumes[] | {id: .VolumeId, state: .State, size: .Size}] | sort_by(.id)' \
    aws ec2 describe-volumes --filters Name=tag:project,Values=amber

section "Application Auto Scaling: prd-amber の ECS の対象 (ない)"
query '[.ScalableTargets[] | select(.ResourceId | contains("prd-amber")) | .ResourceId] | sort' \
    aws application-autoscaling describe-scalable-targets --service-namespace ecs

finish
