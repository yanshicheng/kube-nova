package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddSysUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysUserLogic {
	return &AddSysUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysUserLogic) AddSysUser(req *types.AddSysUserRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("添加用户请求: operator=%s, username=%s", username, req.Username)

	// 调用 RPC 服务添加用户
	_, err = l.svcCtx.PortalRpc.UserAdd(l.ctx, &pb.AddSysUserReq{
		Username:   req.Username,
		Nickname:   req.Nickname,
		Email:      req.Email,
		Phone:      req.Phone,
		WorkNumber: req.WorkNumber,
		DeptId:     req.DeptId,
		DingtalkId: req.DingtalkId,
		WechatId:   req.WechatId,
		FeishuId:   req.FeishuId,
		CreatedBy:  username,
		UpdatedBy:  username,
	})
	if err != nil {
		l.Errorf("添加用户失败: operator=%s, username=%s, error=%v", username, req.Username, err)
		return "", err
	}

	l.Infof("添加用户成功: operator=%s, username=%s", username, req.Username)
	return "添加用户成功", nil
}
