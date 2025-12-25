package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListReplicationPoliciesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出复制策略
func NewListReplicationPoliciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReplicationPoliciesLogic {
	return &ListReplicationPoliciesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListReplicationPoliciesLogic) ListReplicationPolicies(req *types.ListReplicationPoliciesRequest) (resp *types.ListReplicationPoliciesResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListReplicationPolicies(l.ctx, &pb.ListReplicationPoliciesReq{
		RegistryUuid: req.RegistryUuid,
		Name:         req.Name,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.ReplicationPolicy
	for _, policy := range rpcResp.Items {
		var filters []types.ReplicationFilter
		for _, filter := range policy.Filters {
			filters = append(filters, types.ReplicationFilter{
				Type:       filter.Type,
				Value:      filter.Value,
				Decoration: filter.Decoration,
			})
		}

		items = append(items, types.ReplicationPolicy{
			Id:            policy.Id,
			Name:          policy.Name,
			Description:   policy.Description,
			SrcRegistry:   policy.SrcRegistry,
			DestRegistry:  policy.DestRegistry,
			DestNamespace: policy.DestNamespace,
			Filters:       filters,
			Trigger: types.ReplicationTrigger{
				Type:            policy.Trigger.Type,
				TriggerSettings: policy.Trigger.TriggerSettings,
			},
			Deletion:     policy.Deletion,
			Override:     policy.Override,
			Enabled:      policy.Enabled,
			CreationTime: policy.CreationTime,
			UpdateTime:   policy.UpdateTime,
		})
	}

	l.Infof("复制策略列表查询成功: Total=%d", rpcResp.Total)
	return &types.ListReplicationPoliciesResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
