package portalservicelogic

import (
	"context"
	"database/sql"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateLogic {
	return &UserUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserUpdate 更新用户信息
func (l *UserUpdateLogic) UserUpdate(in *pb.UpdateSysUserReq) (*pb.UpdateSysUserResp, error) {

	// 参数验证
	if in.Id <= 0 || in.UpdatedBy == "" || in.Email == "" || in.Phone == "" || in.WorkNumber == "" || in.Status <= 0 {
		l.Error("参数验证失败")
		return nil, errorx.Msg("参数验证失败")
	}

	// 验证用户是否存在
	existUser, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("用户不存在，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}

	// 如果指定了部门，验证部门是否存在
	if in.DeptId > 0 {
		_, err := l.svcCtx.SysDept.FindOne(l.ctx, in.DeptId)
		if err != nil {
			l.Errorf("部门不存在，部门ID: %d, 错误: %v", in.DeptId, err)
			return nil, errorx.Msg("部门不存在")
		}
	}

	// 更新用户信息
	existUser.Nickname = in.Nickname
	existUser.Email = in.Email
	existUser.Phone = in.Phone
	existUser.WorkNumber = in.WorkNumber
	existUser.DeptId = in.DeptId
	existUser.Status = in.Status
	existUser.IsNeedResetPwd = in.IsNeedResetPwd
	existUser.UpdatedBy = in.UpdatedBy
	existUser.DingtalkId = sql.NullString{String: in.DingtalkId, Valid: in.DingtalkId != ""}
	existUser.WechatId = sql.NullString{String: in.WechatId, Valid: in.WechatId != ""}
	existUser.FeishuId = sql.NullString{String: in.FeishuId, Valid: in.FeishuId != ""}

	// 更新到数据库
	err = l.svcCtx.SysUser.Update(l.ctx, existUser)
	if err != nil {
		l.Errorf("更新用户信息失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新用户信息失败")
	}

	l.Infof("用户信息更新成功，用户ID: %d, 用户名: %s", in.Id, existUser.Username)
	return &pb.UpdateSysUserResp{}, nil
}
