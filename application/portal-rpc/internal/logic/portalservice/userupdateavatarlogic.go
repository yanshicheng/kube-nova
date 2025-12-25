package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserUpdateAvatarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserUpdateAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateAvatarLogic {
	return &UserUpdateAvatarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改头像
func (l *UserUpdateAvatarLogic) UserUpdateAvatar(in *pb.UpdateSysUserAvatarReq) (*pb.UpdateSysUserAvatarResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}
	if in.Avatar == "" {
		l.Error("头像地址不能为空")
		return nil, errorx.Msg("头像地址不能为空")
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
	// 判断 avatar 是不是 / 开头 如果不是则补全
	if in.Avatar[0] != '/' {
		in.Avatar = "/" + in.Avatar
	}
	// 更新用户头像
	existUser.Avatar = in.Avatar
	existUser.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysUser.Update(l.ctx, existUser)
	if err != nil {
		l.Errorf("更新用户头像失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新头像失败")
	}

	l.Infof("用户头像更新成功，用户ID: %d, 头像地址: %s", in.Id, in.Avatar)
	return &pb.UpdateSysUserAvatarResp{}, nil
}
