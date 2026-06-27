// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsTektonRuntimeResourceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 Tekton 运行时资源
func NewDevopsTektonRuntimeResourceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonRuntimeResourceListLogic {
	return &DevopsTektonRuntimeResourceListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonRuntimeResourceListLogic) DevopsTektonRuntimeResourceList(req *types.ListTektonRuntimeResourceRequest) (resp *types.ListTektonRuntimeResourceResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.TektonRuntimeResourceList(l.ctx, &executionservice.ListTektonRuntimeResourceReq{
		ResourceType:          req.ResourceType,
		ProjectId:             req.ProjectId,
		SystemId:              req.SystemId,
		EnvironmentId:         req.EnvironmentId,
		BuildChannelBindingId: req.BuildChannelBindingId,
		Keyword:               req.Keyword,
		CurrentUserId:         currentUserID(l.ctx),
		CurrentRoles:          currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.TektonRuntimeResource, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, tektonRuntimeResourceToType(item))
	}
	return &types.ListTektonRuntimeResourceResponse{Items: items, Total: result.Total}, nil
}
