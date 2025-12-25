package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysUserByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysUserByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysUserByIdLogic {
	return &GetSysUserByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysUserByIdLogic) GetSysUserById(req *types.DefaultIdRequest) (resp *types.SysUser, err error) {

	// 调用 RPC 服务获取用户信息
	res, err := l.svcCtx.PortalRpc.UserGetById(l.ctx, &pb.GetSysUserByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取用户失败: userId=%d, error=%v", req.Id, err)
		return nil, err
	}

	l.Infof("获取用户成功: userId=%d, username=%s", req.Id, res.Data.Username)

	return &types.SysUser{
		Id:             res.Data.Id,
		Username:       res.Data.Username,
		Nickname:       res.Data.Nickname,
		Avatar:         res.Data.Avatar,
		Email:          res.Data.Email,
		Phone:          res.Data.Phone,
		WorkNumber:     res.Data.WorkNumber,
		DeptId:         res.Data.DeptId,
		Status:         res.Data.Status,
		IsNeedResetPwd: res.Data.IsNeedResetPwd,
		CreatedBy:      res.Data.CreatedBy,
		UpdatedBy:      res.Data.UpdatedBy,
		CreatedAt:      res.Data.CreatedAt,
		UpdatedAt:      res.Data.UpdatedAt,
	}, nil
}
