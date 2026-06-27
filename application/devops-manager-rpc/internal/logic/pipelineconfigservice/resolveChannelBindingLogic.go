package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveChannelBindingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveChannelBindingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveChannelBindingLogic {
	return &ResolveChannelBindingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveChannelBindingLogic) ResolveChannelBinding(in *pb.ResolveChannelBindingReq) (*pb.ResolveChannelBindingResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析渠道绑定失败: %v", err)
		return nil, err
	}
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, in.ChannelBindingId)
	if err != nil {
		l.Errorf("解析渠道绑定失败: %v", err)
		return nil, err
	}
	if binding.ProjectID != in.ProjectId {
		l.Errorf("渠道绑定不属于当前项目")
		return nil, errorx.Msg("渠道绑定不属于当前项目")
	}
	if binding.Status != 1 {
		l.Errorf("渠道绑定已停用")
		return nil, errorx.Msg("渠道绑定已停用")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		l.Errorf("解析渠道绑定失败: %v", err)
		return nil, err
	}
	if channel.Status != 1 {
		l.Errorf("渠道已停用")
		return nil, errorx.Msg("渠道已停用")
	}
	credential, credentialReady, credentialMessage, err := resolveBindingCredential(l.ctx, l.svcCtx, in.ProjectId, binding, channel)
	if err != nil {
		l.Errorf("解析渠道绑定失败: %v", err)
		return nil, err
	}
	return &pb.ResolveChannelBindingResp{
		Binding:           projectChannelBindingToPb(binding),
		Channel:           channelToPb(channel),
		Credential:        credential,
		CredentialReady:   credentialReady,
		CredentialMessage: credentialMessage,
	}, nil
}
