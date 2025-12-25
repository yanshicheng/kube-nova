package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserUpdateInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserUpdateInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateInfoLogic {
	return &UserUpdateInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 个人修改信息
func (l *UserUpdateInfoLogic) UserUpdateInfo(in *pb.UpdateSysUserInfoReq) (*pb.UpdateSysUserInfoResp, error) {
	// 参数验证
	if in.Id <= 0 || in.Nickname == "" || in.Email == "" || in.Phone == "" || in.UpdatedBy == "" {
		l.Error("参数验证失败")
		return nil, errorx.Msg("参数验证失败")
	}

	// 验证用户是否存在
	existUser, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("用户不存在，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}

	// 更新用户个人信息
	existUser.Nickname = in.Nickname
	existUser.Email = in.Email
	existUser.Phone = in.Phone
	existUser.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysUser.Update(l.ctx, existUser)
	if err != nil {
		l.Errorf("更新用户个人信息失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新个人信息失败")
	}

	l.Infof("用户个人信息更新成功，用户ID: %d, 用户名: %s", in.Id, existUser.Username)
	return &pb.UpdateSysUserInfoResp{}, nil
}
