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

type DevopsPipelineGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 流水线详情
func NewDevopsPipelineGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineGetLogic {
	return &DevopsPipelineGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineGetLogic) DevopsPipelineGet(req *types.DefaultStringIdRequest) (resp *types.DevopsPipeline, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineGet(l.ctx, &executionservice.GetPipelineReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := pipelineToType(result.Data)

	return &data, nil
}
