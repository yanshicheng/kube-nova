package portalservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserResetPwdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserResetPwdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserResetPwdLogic {
	return &UserResetPwdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 重置密码
func (l *UserResetPwdLogic) UserResetPwd(in *pb.ResetSysUserPwdReq) (*pb.ResetSysUserPwdResp, error) {
	// 参数验证
	if in.Id == 0 || in.UpdatedBy == "" {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 验证用户是否存在
	existUser, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("用户不存在，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}
	encryptPassword, err := utils.EncryptPassword(utils.GeneratePassword())
	if err != nil {
		l.Errorf("密码加密失败，错误: %v", err)
		return nil, errorx.Msg("密码加密失败")
	}
	defaultPassword := encryptPassword
	existUser.IsNeedResetPwd = 1 // 标记需要重置密码
	existUser.UpdatedAt = time.Now()
	existUser.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysUser.Update(l.ctx, existUser)
	if err != nil {
		l.Errorf("重置用户密码失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("重置密码失败")
	}

	l.Infof("用户密码重置成功，用户ID: %d, 用户名: %s, 默认密码: %s", in.Id, existUser.Username, defaultPassword)
	return &pb.ResetSysUserPwdResp{}, nil
}
