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

type DevopsPipelineRunDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除流水线执行记录
func NewDevopsPipelineRunDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunDeleteLogic {
	return &DevopsPipelineRunDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunDeleteLogic) DevopsPipelineRunDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineExecRpc.PipelineRunDelete(l.ctx, &executionservice.DeletePipelineRunReq{
		RunId:         req.Id,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
