package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserDelLogic {
	return &UserDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UserDelLogic) UserDel(in *pb.DelSysUserReq) (*pb.DelSysUserResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 验证用户是否存在
	existUser, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("用户不存在，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}

	// 检查是否为管理员账号，防止误删
	if existUser.Username == "admin" || existUser.Username == "root" || existUser.Username == "system" || existUser.Username == "super_admin" {
		l.Error("不能删除管理员账号")
		return nil, errorx.Msg("不能删除管理员账号")
	}

	// 执行软删除
	err = l.svcCtx.SysUser.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除用户失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除用户失败")
	}

	l.Infof("用户删除成功，用户ID: %d, 用户名: %s", in.Id, existUser.Username)
	return &pb.DelSysUserResp{}, nil
}
