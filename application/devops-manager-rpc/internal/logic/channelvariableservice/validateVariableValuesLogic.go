package channelvariableservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateVariableValuesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateVariableValuesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateVariableValuesLogic {
	return &ValidateVariableValuesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateVariableValuesLogic) ValidateVariableValues(in *pb.ValidateVariableValuesReq) (*pb.ValidateVariableValuesResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ValidateVariableValuesResp{}, nil
}
