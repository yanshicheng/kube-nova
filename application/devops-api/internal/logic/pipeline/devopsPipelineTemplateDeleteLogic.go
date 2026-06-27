// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineTemplateDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除流水线模板
func NewDevopsPipelineTemplateDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateDeleteLogic {
	return &DevopsPipelineTemplateDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateDeleteLogic) DevopsPipelineTemplateDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineRpc.PipelineTemplateDelete(l.ctx, &pipelineconfigservice.DeleteByIdReq{
		Id:            req.Id,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
