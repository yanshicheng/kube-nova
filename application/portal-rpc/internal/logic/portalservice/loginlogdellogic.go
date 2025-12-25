package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogDelLogic {
	return &LoginLogDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------登录日志表-----------------------
func (l *LoginLogDelLogic) LoginLogDel(in *pb.DelSysLoginLogReq) (*pb.DelSysLoginLogResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("删除登录日志失败：日志ID无效")
		return nil, errorx.Msg("登录日志ID无效")
	}

	// 检查登录日志是否存在
	existingLog, err := l.svcCtx.SysLoginLog.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("删除登录日志失败：登录日志不存在, logId: %d", in.Id)
			return nil, errorx.Msg("登录日志不存在")
		}
		l.Errorf("查询登录日志信息失败: %v", err)
		return nil, errorx.Msg("查询登录日志信息失败")
	}

	// 直接删除登录日志
	err = l.svcCtx.SysLoginLog.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除登录日志失败: %v", err)
		return nil, errorx.Msg("删除登录日志失败")
	}

	l.Infof("删除登录日志成功，日志ID: %d, 用户名: %s, 登录时间: %s",
		in.Id, existingLog.Username, existingLog.LoginTime.Format("2006-01-02 15:04:05"))
	return &pb.DelSysLoginLogResp{}, nil
}
