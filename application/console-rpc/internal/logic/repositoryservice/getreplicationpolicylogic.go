package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetReplicationPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReplicationPolicyLogic {
	return &GetReplicationPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetReplicationPolicyLogic) GetReplicationPolicy(in *pb.GetReplicationPolicyReq) (*pb.GetReplicationPolicyResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	policy, err := client.Replication().Get(in.PolicyId)
	if err != nil {
		return nil, errorx.Msg("获取复制策略失败")
	}

	resp := &pb.GetReplicationPolicyResp{
		Data: &pb.ReplicationPolicy{
			Id:            policy.ID,
			Name:          policy.Name,
			Description:   policy.Description,
			DestNamespace: policy.DestNamespace,
			Deletion:      policy.Deletion,
			Override:      policy.Override,
			Enabled:       policy.Enabled,
			CreationTime:  policy.CreationTime.Unix(),
			UpdateTime:    policy.UpdateTime.Unix(),
		},
	}

	// srcRegistry 和 destRegistry 是 int64 ID
	if policy.SrcRegistry != nil {
		resp.Data.SrcRegistry = policy.SrcRegistry.ID
	}
	if policy.DestRegistry != nil {
		resp.Data.DestRegistry = policy.DestRegistry.ID
	}

	// 转换过滤器
	for _, filter := range policy.Filters {
		resp.Data.Filters = append(resp.Data.Filters, &pb.ReplicationFilter{
			Type:       filter.Type,
			Value:      filter.Value,
			Decoration: filter.Decoration,
		})
	}

	// 转换触发器
	if policy.Trigger != nil {
		resp.Data.Trigger = &pb.ReplicationTrigger{
			Type:            policy.Trigger.Type,
			TriggerSettings: policy.Trigger.TriggerSettings,
		}
	}

	return resp, nil
}
