package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateReplicationPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建复制策略
func NewCreateReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReplicationPolicyLogic {
	return &CreateReplicationPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateReplicationPolicyLogic) CreateReplicationPolicy(req *types.CreateReplicationPolicyRequest) (resp string, err error) {
	var filters []*pb.ReplicationFilter
	for _, filter := range req.Filters {
		filters = append(filters, &pb.ReplicationFilter{
			Type:       filter.Type,
			Value:      filter.Value,
			Decoration: filter.Decoration,
		})
	}

	rpcResp, err := l.svcCtx.RepositoryRpc.CreateReplicationPolicy(l.ctx, &pb.CreateReplicationPolicyReq{
		RegistryUuid:  req.RegistryUuid,
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

	l.Infof("复制策略创建成功: PolicyId=%d", rpcResp.Id)
	return "复制策略创建成功", nil
}
