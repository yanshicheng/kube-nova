package channelvariableservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RenderOutputLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRenderOutputLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RenderOutputLogic {
	return &RenderOutputLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RenderOutputLogic) RenderOutput(in *pb.RenderOutputReq) (*pb.RenderOutputResp, error) {
	// todo: add your logic here and delete this line

	return &pb.RenderOutputResp{}, nil
}
