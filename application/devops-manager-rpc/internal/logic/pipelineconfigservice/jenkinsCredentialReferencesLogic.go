package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsCredentialReferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsCredentialReferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsCredentialReferencesLogic {
	return &JenkinsCredentialReferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsCredentialReferencesLogic) JenkinsCredentialReferences(in *pb.JenkinsCredentialReferenceReq) (*pb.ListJenkinsCredentialReferencesResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("Jenkins 凭证引用查询失败: %v", err)
		return nil, err
	}
	if _, err := ensureJenkinsBuildBinding(l.ctx, l.svcCtx, in.ProjectId, in.BuildChannelBindingId); err != nil {
		l.Errorf("Jenkins 凭证引用查询失败: %v", err)
		return nil, err
	}
	data, total, err := collectJenkinsCredentialReferences(l.ctx, l.svcCtx, in.ProjectId, in.BuildChannelBindingId, in.CredentialId)
	if err != nil {
		l.Errorf("Jenkins 凭证引用查询失败: %v", err)
		return nil, err
	}

	return &pb.ListJenkinsCredentialReferencesResp{Data: data, Total: total}, nil
}
