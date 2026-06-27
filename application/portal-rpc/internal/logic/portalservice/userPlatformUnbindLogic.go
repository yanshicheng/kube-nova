package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserPlatformUnbindLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserPlatformUnbindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPlatformUnbindLogic {
	return &UserPlatformUnbindLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserPlatformUnbind 解绑用户平台
func (l *UserPlatformUnbindLogic) UserPlatformUnbind(_ *pb.UnbindUserPlatformReq) (*pb.UnbindUserPlatformResp, error) {
	return nil, errorx.Msg("平台授权请在项目管理中维护")
}
