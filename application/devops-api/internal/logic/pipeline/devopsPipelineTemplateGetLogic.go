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

type DevopsPipelineTemplateGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取流水线模板详情
func NewDevopsPipelineTemplateGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateGetLogic {
	return &DevopsPipelineTemplateGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateGetLogic) DevopsPipelineTemplateGet(req *types.DefaultStringIdRequest) (resp *types.DevopsPipelineTemplate, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineTemplateGet(l.ctx, &pipelineconfigservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := pipelineTemplateToType(result.Data)

	return &data, nil
}
