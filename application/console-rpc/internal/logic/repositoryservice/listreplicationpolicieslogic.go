package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListReplicationPoliciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListReplicationPoliciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReplicationPoliciesLogic {
	return &ListReplicationPoliciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 复制策略管理 ============
func (l *ListReplicationPoliciesLogic) ListReplicationPolicies(in *pb.ListReplicationPoliciesReq) (*pb.ListReplicationPoliciesResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := types.ListRequest{
		Search:   in.Name,
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	resp, err := client.Replication().List(req)
	if err != nil {
		return nil, errorx.Msg("查询复制策略列表失败")
	}

	var items []*pb.ReplicationPolicy
	for _, policy := range resp.Items {
		item := &pb.ReplicationPolicy{
			Id:            policy.ID,
			Name:          policy.Name,
			Description:   policy.Description,
			DestNamespace: policy.DestNamespace,
			Deletion:      policy.Deletion,
			Override:      policy.Override,
			Enabled:       policy.Enabled,
			CreationTime:  policy.CreationTime.Unix(),
			UpdateTime:    policy.UpdateTime.Unix(),
		}

		// srcRegistry 和 destRegistry 是 int64 ID
		if policy.SrcRegistry != nil {
			item.SrcRegistry = policy.SrcRegistry.ID
		}
		if policy.DestRegistry != nil {
			item.DestRegistry = policy.DestRegistry.ID
		}

		// 转换过滤器
		for _, filter := range policy.Filters {
			item.Filters = append(item.Filters, &pb.ReplicationFilter{
				Type:       filter.Type,
				Value:      filter.Value,
				Decoration: filter.Decoration,
			})
		}

		// 转换触发器
		if policy.Trigger != nil {
			item.Trigger = &pb.ReplicationTrigger{
				Type:            policy.Trigger.Type,
				TriggerSettings: policy.Trigger.TriggerSettings,
			}
		}

		items = append(items, item)
	}

	return &pb.ListReplicationPoliciesResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
