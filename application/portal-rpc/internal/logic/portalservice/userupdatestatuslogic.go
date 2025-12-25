package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type UserUpdateStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserUpdateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateStatusLogic {
	return &UserUpdateStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 禁用或启用用户
func (l *UserUpdateStatusLogic) UserUpdateStatus(in *pb.UpdateSysUserStatusReq) (*pb.UpdateSysUserStatusResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}
	if in.Status < 0 || in.Status > 1 {
		l.Error("用户状态值无效")
		return nil, errorx.Msg("用户状态值无效")
	}
	if in.UpdatedBy == "" {
		l.Error("更新人ID不能为空")
		return nil, errorx.Msg("更新人ID不能为空")
	}

	// 验证用户是否存在
	existUser, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("用户不存在，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}

	// 更新用户状态
	existUser.Status = in.Status
	existUser.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysUser.Update(l.ctx, existUser)
	if err != nil {
		l.Errorf("更新用户状态失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新用户状态失败")
	}

	statusText := "禁用"
	if in.Status == 1 {
		statusText = "启用"
	}

	l.Infof("用户状态更新成功，用户ID: %d, 用户名: %s, 状态: %s", in.Id, existUser.Username, statusText)
	return &pb.UpdateSysUserStatusResp{}, nil
}
