package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListAllCanariesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出所有 Canary
func NewListAllCanariesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAllCanariesLogic {
	return &ListAllCanariesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAllCanariesLogic) ListAllCanaries(req *types.ListAllCanariesRequest) (resp *types.ListAllCanariesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
