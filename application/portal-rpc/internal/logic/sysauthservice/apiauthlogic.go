package sysauthservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApiAuthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApiAuthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApiAuthLogic {
	return &ApiAuthLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApiAuthLogic) ApiAuth(in *pb.ApiAuthRequest) (*pb.ApiAuthResponse, error) {
	if l.svcCtx.Config.DemoMode {
		if in.ApiMethod != "GET" && in.ApiPath != "/portal/v1/auth/logout" {
			return nil, errorx.Msg("演示模式下仅允许 GET 请求!")
		}
		//if in.ApiPath == "/ws/v1/pod/exec/terminal" {
		//	return nil, errorx.Msg("演示模式下不允许访问终端!")
		//}
	}

	permission, err := l.svcCtx.AuthzManager.CheckPermission(l.ctx, in.UserRoles, in.ApiPath, in.ApiMethod)
	if err != nil {
		l.Errorf("[RBAC] 检查权限失败: %v", err)
		return nil, errorx.Msg("检查权限失败")
	}
	if !permission {
		l.Errorf("[RBAC] 用户无权限访问: %s %s", in.ApiMethod, in.ApiPath)
		return nil, errorx.Msg("无权限访问")
	}
	return &pb.ApiAuthResponse{
		IsAuth: permission,
	}, nil
}
