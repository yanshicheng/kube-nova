package user

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysUserInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysUserInfoLogic {
	return &GetSysUserInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// user/getSysUserInfoLogic.go
func (l *GetSysUserInfoLogic) GetSysUserInfo() (resp *types.SysUserInfo, err error) {
	// 获取上下文中的用户信息
	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		logx.WithContext(l.ctx).Error("获取用户信息失败: 未找到用户ID")
		return nil, errors.New("未找到用户信息")
	}

	// 调用 RPC 服务获取当前用户信息
	res, err := l.svcCtx.PortalRpc.UserGetInfo(l.ctx, &pb.GetSysUserInfoReq{
		Id: userId,
	})
	if err != nil {
		l.Errorf("获取当前用户信息失败: userId=%d, error=%v", userId, err)
		return nil, err
	}

	return &types.SysUserInfo{
		Id:             res.Id,
		Username:       res.Username,
		Nickname:       res.Nickname,
		Avatar:         res.Avatar,
		Email:          res.Email,
		Phone:          res.Phone,
		WorkNumber:     res.WorkNumber,
		Status:         res.Status,
		IsNeedResetPwd: res.IsNeedResetPwd,
		CreatedBy:      res.CreatedBy,
		UpdatedBy:      res.UpdatedBy,
		CreatedAt:      res.CreatedAt,
		UpdatedAt:      res.UpdatedAt,
		DeptNames:      res.DeptNames,
		RoleNames:      res.RoleNames,
	}, nil
}
