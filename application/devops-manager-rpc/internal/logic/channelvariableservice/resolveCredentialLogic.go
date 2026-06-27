package channelvariableservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveCredentialLogic {
	return &ResolveCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveCredentialLogic) ResolveCredential(in *pb.ResolveCredentialReq) (*pb.ResolveCredentialResp, error) {
	// todo: add your logic here and delete this line

	return &pb.ResolveCredentialResp{}, nil
}
