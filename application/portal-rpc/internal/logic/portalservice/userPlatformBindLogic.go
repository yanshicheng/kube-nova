package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserPlatformBindLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserPlatformBindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPlatformBindLogic {
	return &UserPlatformBindLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------用户平台权限表-----------------------
// UserPlatformBind 绑定用户平台
func (l *UserPlatformBindLogic) UserPlatformBind(_ *pb.BindUserPlatformReq) (*pb.BindUserPlatformResp, error) {
	return nil, errorx.Msg("平台授权请在项目管理中维护")
}
