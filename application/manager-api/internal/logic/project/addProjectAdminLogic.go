package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddProjectAdminLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewAddProjectAdminLogic 批量添加项目管理员，使用事务先删除所有管理员再批量添加
func NewAddProjectAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectAdminLogic {
	return &AddProjectAdminLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddProjectAdminLogic) AddProjectAdmin(req *types.AddProjectAdminRequest) (resp string, err error) {
	return "", errorx.Msg("项目成员请在门户管理中维护")
}
