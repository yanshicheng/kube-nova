package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateReplicationPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新复制策略
func NewUpdateReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateReplicationPolicyLogic {
	return &UpdateReplicationPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateReplicationPolicyLogic) UpdateReplicationPolicy(req *types.UpdateReplicationPolicyRequest) (resp string, err error) {
	var filters []*pb.ReplicationFilter
	for _, filter := range req.Filters {
		filters = append(filters, &pb.ReplicationFilter{
			Type:       filter.Type,
			Value:      filter.Value,
			Decoration: filter.Decoration,
		})
	}

	_, err = l.svcCtx.RepositoryRpc.UpdateReplicationPolicy(l.ctx, &pb.UpdateReplicationPolicyReq{
		RegistryUuid:  req.RegistryUuid,
		PolicyId:      req.PolicyId,
		Name:          req.Name,
		Description:   req.Description,
		SrcRegistry:   req.SrcRegistry,
		DestRegistry:  req.DestRegistry,
		DestNamespace: req.DestNamespace,
		Filters:       filters,
		Trigger: &pb.ReplicationTrigger{
			Type:            req.Trigger.Type,
			TriggerSettings: req.Trigger.TriggerSettings,
		},
		Deletion: req.Deletion,
		Override: req.Override,
		Enabled:  req.Enabled,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("复制策略更新成功: PolicyId=%d", req.PolicyId)
	return "复制策略更新成功", nil
}
