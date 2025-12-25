package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/code"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserUpdatePwdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserUpdatePwdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdatePwdLogic {
	return &UserUpdatePwdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改密码
func (l *UserUpdatePwdLogic) UserUpdatePwd(in *pb.UpdateSysUserPwdReq) (*pb.UpdateSysUserPwdResp, error) {
	// 参数验证
	if in.Username == "" {
		l.Error("用户名称不能为空")
		return nil, errorx.Msg("用户名称不能为空")
	}
	if in.OldPassword == "" {
		l.Error("旧密码不能为空")
		return nil, errorx.Msg("旧密码不能为空")
	}
	if in.NewPassword == "" {
		l.Error("新密码不能为空")
		return nil, errorx.Msg("新密码不能为空")
	}
	if in.ConfirmPassword == "" {
		l.Error("确认密码不能为空")
		return nil, errorx.Msg("确认密码不能为空")
	}
	if in.NewPassword != in.ConfirmPassword {
		l.Error("新密码和确认密码不一致")
		return nil, errorx.Msg("新密码和确认密码不一致")
	}

	// 验证用户是否存在
	existUser, err := l.svcCtx.SysUser.FindOneByUsername(l.ctx, in.Username)
	if err != nil {
		l.Errorf("用户不存在，用户名称: %d, 错误: %v", in.Username, err)
		return nil, errorx.Msg("用户不存在")
	}

	// 验证旧密码（实际项目中应该加密验证）
	oldPwd, err := utils.DecodeBase64Password(in.OldPassword)
	if err != nil {
		l.Errorf("解码密码失败: %v", err)
		return nil, errorx.Msg("解码密码失败")
	}
	if !utils.CheckPasswordHash(oldPwd, existUser.Password) {
		l.Errorf("旧密码错误: %v", err)
		return nil, errorx.Msg("旧密码错误")
	}
	newPwd, err := utils.DecodeBase64Password(in.NewPassword)
	if err != nil {
		l.Errorf("解码密码失败: %v", err)
		return nil, errorx.Msg("解码密码失败")
	}
	confirmNewPassword, err := utils.DecodeBase64Password(in.ConfirmPassword)
	if err != nil {
		l.Errorf("解码密码失败: %v", err)
		return nil, errorx.Msg("解码密码失败")
	}
	if newPwd != confirmNewPassword {
		l.Errorf("密码不匹配: %v, %v", newPwd, confirmNewPassword)
		return nil, errorx.Msg("密码不匹配")
	}
	if oldPwd == newPwd {
		l.Errorf("新旧密码一致: %v", newPwd)
		return nil, errorx.Msg("新密码不能和旧密码一致")
	}
	if !checkPassword(newPwd) {
		l.Errorf("密码不符合规则: %v", newPwd)
		return nil, code.PasswordIllegal
	}
	// 密码加密修改密码
	newPwdHash, err := utils.EncryptPassword(newPwd)
	if err != nil {
		l.Errorf("密码加密失败: %v", err)
		return nil, errorx.Msg("密码加密失败")
	}
	// 更新密码（实际项目中应该加密）
	existUser.Password = newPwdHash
	existUser.IsNeedResetPwd = 0 // 更新密码后不需要重置密码
	existUser.UpdatedBy = in.UpdatedBy

	// 更新到数据库
	err = l.svcCtx.SysUser.Update(l.ctx, existUser)
	if err != nil {
		l.Errorf("更新用户密码失败，用户名称: %d, 错误: %v", in.Username, err)
		return nil, errorx.Msg("更新密码失败")
	}

	l.Infof("用户密码更新成功，用户名称: %d, 用户名: %s", in.Username, existUser.Username)
	return &pb.UpdateSysUserPwdResp{}, nil
}

// 校验函数，必须大于六位，必须包含大小写，数字，特殊字符
// checkPassword 校验密码是否符合规则：必须大于六位，包含大小写字母、数字、特殊字符
func checkPassword(password string) bool {
	// 密码长度必须大于 6 位
	if len(password) <= 6 {
		return false
	}

	// 初始化标志位，用于校验是否包含大写、小写、数字和特殊字符
	var hasUpper, hasLower, hasNumber, hasSpecial bool

	// 遍历密码中的每个字符，进行分类判断
	for _, char := range password {
		// 判断是否是大写字母
		if char >= 'A' && char <= 'Z' {
			hasUpper = true
		} else if char >= 'a' && char <= 'z' { // 判断是否是小写字母
			hasLower = true
		} else if char >= '0' && char <= '9' { // 判断是否是数字
			hasNumber = true
		} else { // 其余字符认为是特殊字符
			hasSpecial = true
		}
	}

	// 只有当所有条件都满足时，返回 true，否则返回 false
	return hasUpper && hasLower && hasNumber && hasSpecial
}
