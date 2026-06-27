package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialTypeOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialTypeOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialTypeOptionsLogic {
	return &CredentialTypeOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialTypeOptionsLogic) CredentialTypeOptions(in *pb.EmptyResp) (*pb.CredentialTypeOptionsResp, error) {
	return &pb.CredentialTypeOptionsResp{Data: credentialTypeOptions()}, nil
}
