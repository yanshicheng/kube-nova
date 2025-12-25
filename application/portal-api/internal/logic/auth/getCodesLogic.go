package auth

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCodesLogic {
	return &GetCodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCodesLogic) GetCodes() (resp []string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "unknown"
	}

	// 这里需要根据实际业务逻辑获取权限码
	// 暂时返回空列表，需要根据实际的权限管理逻辑实现
	codes := []string{}

	return codes, nil
}
