package channelvariableservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveAddressLogic {
	return &ResolveAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveAddressLogic) ResolveAddress(in *pb.ResolveAddressReq) (*pb.ResolveAddressResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ResolveAddressResp{}, nil
}
