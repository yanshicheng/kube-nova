package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsCredentialSyncListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsCredentialSyncListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsCredentialSyncListLogic {
	return &JenkinsCredentialSyncListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsCredentialSyncListLogic) JenkinsCredentialSyncList(in *pb.ListJenkinsCredentialSyncReq) (*pb.ListJenkinsCredentialSyncResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("Jenkins 凭证同步记录查询失败: %v", err)
		return nil, err
	}
	if _, err := ensureJenkinsBuildBinding(l.ctx, l.svcCtx, in.ProjectId, in.BuildChannelBindingId); err != nil {
		l.Errorf("Jenkins 凭证同步记录查询失败: %v", err)
		return nil, err
	}
	items, total, err := l.svcCtx.CredentialSyncModel.List(l.ctx, model.DevopsCredentialSyncListFilter{
		ProjectID:             in.ProjectId,
		BuildChannelBindingID: in.BuildChannelBindingId,
		Page:                  in.Page,
		PageSize:              in.PageSize,
	})
	if err != nil {
		l.Errorf("Jenkins 凭证同步记录查询失败: %v", err)
		return nil, err
	}
	data := make([]*pb.JenkinsCredentialSyncRecord, 0, len(items))
	for _, item := range items {
		record, err := jenkinsCredentialSyncRecordToPb(l.ctx, l.svcCtx, item)
		if err != nil {
			l.Errorf("Jenkins 凭证同步记录转换失败: %v", err)
			return nil, err
		}
		data = append(data, record)
	}

	return &pb.ListJenkinsCredentialSyncResp{Data: data, Total: total}, nil
}
