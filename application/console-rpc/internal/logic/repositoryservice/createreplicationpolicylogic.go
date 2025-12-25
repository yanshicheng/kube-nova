package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateReplicationPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReplicationPolicyLogic {
	return &CreateReplicationPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateReplicationPolicyLogic) CreateReplicationPolicy(in *pb.CreateReplicationPolicyReq) (*pb.CreateReplicationPolicyResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	policy := &types.ReplicationPolicy{
		Name:          in.Name,
		Description:   in.Description,
		DestNamespace: in.DestNamespace,
		Deletion:      in.Deletion,
		Override:      in.Override,
		Enabled:       in.Enabled,
	}

	// srcRegistry 和 destRegistry 使用 ID
	if in.SrcRegistry > 0 {
		policy.SrcRegistry = &types.Registry{ID: in.SrcRegistry}
	}
	if in.DestRegistry > 0 {
		policy.DestRegistry = &types.Registry{ID: in.DestRegistry}
	}

	// 转换过滤器
	for _, filter := range in.Filters {
		policy.Filters = append(policy.Filters, types.ReplicationFilter{
			Type:       filter.Type,
			Value:      filter.Value,
			Decoration: filter.Decoration,
		})
	}

	// 转换触发器
	if in.Trigger != nil {
		policy.Trigger = &types.ReplicationTrigger{
			Type:            in.Trigger.Type,
			TriggerSettings: in.Trigger.TriggerSettings,
		}
	}

	policyID, err := client.Replication().Create(policy)
	if err != nil {
		return nil, errorx.Msg("创建复制策略失败")
	}

	return &pb.CreateReplicationPolicyResp{
		Id:      policyID,
		Message: "复制策略创建成功",
	}, nil
}
