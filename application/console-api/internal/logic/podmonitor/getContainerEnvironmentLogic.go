package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetContainerEnvironmentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取容器环境信息
func NewGetContainerEnvironmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContainerEnvironmentLogic {
	return &GetContainerEnvironmentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetContainerEnvironmentLogic) GetContainerEnvironment(req *types.GetContainerEnvironmentRequest) (resp *types.GetContainerEnvironmentResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
