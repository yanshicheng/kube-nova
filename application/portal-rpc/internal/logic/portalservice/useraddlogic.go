package portalservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/common"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserAddLogic {
	return &UserAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------用户表-----------------------
func (l *UserAddLogic) UserAdd(in *pb.AddSysUserReq) (*pb.AddSysUserResp, error) {
	// 参数验证
	if in.Username == "" || in.Nickname == "" || in.Email == "" || in.Phone == "" || in.WorkNumber == "" {
		l.Error("参数验证失败")
		return nil, errorx.Msg("参数验证失败")
	}

	// 检查用户名是否已存在
	existUser, _ := l.svcCtx.SysUser.FindOneByUsername(l.ctx, in.Username)
	if existUser != nil {
		l.Errorf("用户名已存在，用户名: %s", in.Username)
		return nil, errorx.Msg("用户名已存在")
	}

	// 如果指定了部门，验证部门是否存在
	if in.DeptId > 0 {
		_, err := l.svcCtx.SysDept.FindOne(l.ctx, in.DeptId)
		if err != nil {
			l.Errorf("部门不存在，部门ID: %d, 错误: %v", in.DeptId, err)
			return nil, errorx.Msg("部门不存在")
		}
	}
	encryptPassword, err := utils.EncryptPassword(utils.GeneratePassword())
	if err != nil {
		l.Errorf("密码加密失败，错误: %v", err)
		return nil, errorx.Msg("密码加密失败")
	}
	// 构建用户数据
	sysUser := &model.SysUser{
		Username:       in.Username,
		Password:       encryptPassword,
		Nickname:       in.Nickname,
		Avatar:         common.Avatar, // 默认头像
		Email:          in.Email,
		Phone:          in.Phone,
		WorkNumber:     in.WorkNumber,
		DeptId:         in.DeptId,
		Status:         1, // 默认启用
		IsNeedResetPwd: 1, // 需要重置密码
		CreatedBy:      in.CreatedBy,
		UpdatedBy:      in.UpdatedBy,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 插入数据库
	result, err := l.svcCtx.SysUser.Insert(l.ctx, sysUser)
	if err != nil {
		l.Errorf("插入用户失败，错误: %v", err)
		return nil, errorx.Msg("添加用户失败")
	}

	userId, _ := result.LastInsertId()
	l.Infof("用户添加成功，用户ID: %d, 用户名: %s", userId, in.Username)

	return &pb.AddSysUserResp{}, nil
}
