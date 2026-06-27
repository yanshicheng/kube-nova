package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialUsageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialUsageLogic {
	return &CredentialUsageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialUsageLogic) CredentialUsage(in *pb.GetByIdReq) (*pb.GetCredentialUsageResp, error) {
	data, err := l.svcCtx.CredentialModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("凭证使用情况失败: %v", err)
		return nil, err
	}
	if err := ensureCredentialReadAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("凭证使用情况失败: %v", err)
		return nil, err
	}
	items, total, err := collectCredentialUsage(l.ctx, l.svcCtx, in.Id, 200)
	if err != nil {
		l.Errorf("凭证使用情况失败: %v", err)
		return nil, err
	}

	return &pb.GetCredentialUsageResp{Data: items, Total: total}, nil
}

func collectCredentialUsage(ctx context.Context, svcCtx *svc.ServiceContext, credentialID string, limit int64) ([]*pb.CredentialUsage, uint64, error) {
	if _, err := svcCtx.CredentialModel.FindOne(ctx, credentialID); err != nil {
		logx.Errorf("查询凭证使用情况失败: %v", err)
		return nil, 0, err
	}

	items := make([]*pb.CredentialUsage, 0)
	var total uint64

	channels, channelTotal, err := svcCtx.ChannelModel.ListByCredential(ctx, credentialID, limit)
	if err != nil {
		logx.Errorf("查询凭证使用情况失败: %v", err)
		return nil, 0, err
	}
	total += channelTotal
	for _, item := range channels {
		relation := "渠道凭据"
		if item.GlobalCredentialID == credentialID && item.CredentialID == credentialID {
			relation = "渠道凭据、全局凭据"
		} else if item.GlobalCredentialID == credentialID {
			relation = "全局凭据"
		}
		items = append(items, &pb.CredentialUsage{
			ResourceType: "渠道实例",
			ResourceName: item.Name,
			ResourceCode: item.Code,
			ResourceId:   item.ID.Hex(),
			Relation:     relation,
		})
	}

	hosts, hostTotal, err := svcCtx.HostModel.ListByCredential(ctx, credentialID, limit)
	if err != nil {
		logx.Errorf("查询凭证使用情况失败: %v", err)
		return nil, 0, err
	}
	total += hostTotal
	for _, item := range hosts {
		items = append(items, &pb.CredentialUsage{
			ResourceType: "主机资产",
			ResourceName: item.Name,
			ResourceCode: item.IP,
			ResourceId:   item.ID.Hex(),
			Relation:     "登录凭据",
		})
	}

	bindings, bindingTotal, err := svcCtx.ProjectChannelModel.ListByCredential(ctx, credentialID, limit)
	if err != nil {
		logx.Errorf("查询凭证使用情况失败: %v", err)
		return nil, 0, err
	}
	total += bindingTotal
	for _, item := range bindings {
		items = append(items, &pb.CredentialUsage{
			ResourceType: "项目渠道绑定",
			ResourceName: item.ChannelName,
			ResourceCode: item.ChannelCode,
			ResourceId:   item.ID.Hex(),
			Relation:     "项目专用凭据",
		})
	}

	pipelines, pipelineTotal, err := svcCtx.PipelineUsageModel.ListByCredential(ctx, credentialID, limit)
	if err != nil {
		logx.Errorf("查询凭证使用情况失败: %v", err)
		return nil, 0, err
	}
	total += pipelineTotal
	for _, item := range pipelines {
		items = append(items, &pb.CredentialUsage{
			ResourceType: "流水线",
			ResourceName: item.Name,
			ResourceCode: item.Code,
			ResourceId:   item.ID.Hex(),
			Relation:     "流水线步骤凭证",
		})
	}

	if limit > 0 && len(items) > int(limit) {
		items = items[:limit]
	}
	return items, total, nil
}
