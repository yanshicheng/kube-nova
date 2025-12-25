package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetReplicationPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取复制策略
func NewGetReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReplicationPolicyLogic {
	return &GetReplicationPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetReplicationPolicyLogic) GetReplicationPolicy(req *types.GetReplicationPolicyRequest) (resp *types.GetReplicationPolicyResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetReplicationPolicy(l.ctx, &pb.GetReplicationPolicyReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var filters []types.ReplicationFilter
	for _, filter := range rpcResp.Data.Filters {
		filters = append(filters, types.ReplicationFilter{
			Type:       filter.Type,
			Value:      filter.Value,
			Decoration: filter.Decoration,
		})
	}

	l.Infof("复制策略详情查询成功: PolicyId=%d", req.PolicyId)
	return &types.GetReplicationPolicyResponse{
		Data: types.ReplicationPolicy{
			Id:            rpcResp.Data.Id,
			Name:          rpcResp.Data.Name,
			Description:   rpcResp.Data.Description,
			SrcRegistry:   rpcResp.Data.SrcRegistry,
			DestRegistry:  rpcResp.Data.DestRegistry,
			DestNamespace: rpcResp.Data.DestNamespace,
			Filters:       filters,
			Trigger: types.ReplicationTrigger{
				Type:            rpcResp.Data.Trigger.Type,
				TriggerSettings: rpcResp.Data.Trigger.TriggerSettings,
			},
			Deletion:     rpcResp.Data.Deletion,
			Override:     rpcResp.Data.Override,
			Enabled:      rpcResp.Data.Enabled,
			CreationTime: rpcResp.Data.CreationTime,
			UpdateTime:   rpcResp.Data.UpdateTime,
		},
	}, nil
}
