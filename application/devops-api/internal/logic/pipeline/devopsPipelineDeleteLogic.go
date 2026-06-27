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

type DevopsPipelineDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除流水线
func NewDevopsPipelineDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineDeleteLogic {
	return &DevopsPipelineDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineDeleteLogic) DevopsPipelineDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineExecRpc.PipelineDelete(l.ctx, &executionservice.DeletePipelineReq{
		Id:            req.Id,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
