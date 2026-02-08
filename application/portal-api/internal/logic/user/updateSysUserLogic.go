package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysUserLogic {
	return &UpdateSysUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysUserLogic) UpdateSysUser(req *types.UpdateSysUserRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新用户
	_, err = l.svcCtx.PortalRpc.UserUpdate(l.ctx, &pb.UpdateSysUserReq{
		Id:             req.Id,
		Nickname:       req.Nickname,
		Email:          req.Email,
		Phone:          req.Phone,
		WorkNumber:     req.WorkNumber,
		DeptId:         req.DeptId,
		Status:         req.Status,
		IsNeedResetPwd: req.IsNeedResetPwd,
		DingtalkId:     req.DingtalkId,
		WechatId:       req.WechatId,
		FeishuId:       req.FeishuId,
		UpdatedBy:      username,
	})
	if err != nil {
		l.Errorf("更新用户失败: operator=%s, userId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新用户成功: operator=%s, userId=%d", username, req.Id)
	return "更新用户成功", nil
}
