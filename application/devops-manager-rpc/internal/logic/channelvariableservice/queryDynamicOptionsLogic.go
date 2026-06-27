package channelvariableservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryDynamicOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryDynamicOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryDynamicOptionsLogic {
	return &QueryDynamicOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *QueryDynamicOptionsLogic) QueryDynamicOptions(in *pb.QueryDynamicOptionsReq) (*pb.QueryDynamicOptionsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.QueryDynamicOptionsResp{}, nil
}
