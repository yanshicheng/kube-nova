package channelvariableservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetChannelVariableSpecsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetChannelVariableSpecsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetChannelVariableSpecsLogic {
	return &GetChannelVariableSpecsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetChannelVariableSpecsLogic) GetChannelVariableSpecs(in *pb.GetChannelVariableSpecsReq) (*pb.GetChannelVariableSpecsResp, error) {
	// todo: add your logic here and delete this line

	return &pb.GetChannelVariableSpecsResp{}, nil
}
