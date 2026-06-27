package pipelineconfigservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsCredentialSyncDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsCredentialSyncDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsCredentialSyncDeleteLogic {
	return &JenkinsCredentialSyncDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsCredentialSyncDeleteLogic) JenkinsCredentialSyncDelete(in *pb.DeleteJenkinsCredentialSyncReq) (*pb.EmptyResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("Jenkins 凭证副本删除失败: %v", err)
		return nil, err
	}
	if _, err := ensureJenkinsBuildBinding(l.ctx, l.svcCtx, in.ProjectId, in.BuildChannelBindingId); err != nil {
		l.Errorf("Jenkins 凭证副本删除失败: %v", err)
		return nil, err
	}
	record, err := l.svcCtx.CredentialSyncModel.FindOne(l.ctx, in.ProjectId, in.BuildChannelBindingId, in.CredentialId)
	if err != nil {
		l.Errorf("查询 Jenkins 凭证同步记录失败: %v", err)
		return nil, errorx.Msg("Jenkins 凭证同步记录不存在")
	}
	refs, total, err := collectJenkinsCredentialReferences(l.ctx, l.svcCtx, in.ProjectId, in.BuildChannelBindingId, in.CredentialId)
	if err != nil {
		l.Errorf("查询 Jenkins 凭证引用失败: %v", err)
		return nil, err
	}
	if total > 0 {
		l.Errorf("Jenkins 凭证仍被流水线引用，credentialId: %s, referenceCount: %d", in.CredentialId, len(refs))
		return nil, errorx.Msg("Jenkins 凭证仍被流水线引用，不能删除")
	}
	operator := strings.TrimSpace(in.UpdatedBy)
	if operator == "" {
		operator = fmt.Sprint(in.CurrentUserId)
	}
	if strings.TrimSpace(record.JenkinsCredentialID) != "" {
		runtime, err := buildJenkinsSyncRuntime(l.ctx, l.svcCtx, in.ProjectId, in.BuildChannelBindingId)
		if err != nil {
			l.Errorf("构建 Jenkins 同步运行时失败: %v", err)
			return nil, err
		}
		if err := runtime.manager.DeleteFolderCredential(l.ctx, runtime.folder, record.JenkinsCredentialID); err != nil {
			l.Errorf("删除 Jenkins 凭证副本失败: %v", err)
			return nil, errorx.Msg("删除 Jenkins 凭证副本失败")
		}
	}
	if err := l.svcCtx.CredentialSyncModel.MarkDeleted(l.ctx, in.ProjectId, in.BuildChannelBindingId, in.CredentialId, "Jenkins 凭证副本已删除", operator); err != nil {
		l.Errorf("标记 Jenkins 凭证同步记录失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
