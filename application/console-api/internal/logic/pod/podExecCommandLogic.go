package pod

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodExecCommandLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 非交互式执行命令
func NewPodExecCommandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodExecCommandLogic {
	return &PodExecCommandLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodExecCommandLogic) PodExecCommand(req *types.PodExecCommandReq) (resp *types.PodExecCommandResp, err error) {
	// todo: add your logic here and delete this line

	return
}
