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

type DevopsTektonStepImageListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 Tekton 步骤镜像
func NewDevopsTektonStepImageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepImageListLogic {
	return &DevopsTektonStepImageListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepImageListLogic) DevopsTektonStepImageList(req *types.ListTektonStepImageRequest) (resp *types.ListTektonStepImageResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.TektonStepImageList(l.ctx, &pipelineconfigservice.ListTektonStepImageReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Name:       req.Name,
		Code:       req.Code,
		CategoryId: req.CategoryId,
		Status:     req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.TektonStepImageItem, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, tektonStepImageToType(item))
	}

	return &types.ListTektonStepImageResponse{Items: items, Total: result.Total}, nil
}
