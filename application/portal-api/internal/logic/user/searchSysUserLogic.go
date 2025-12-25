package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysUserLogic {
	return &SearchSysUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysUserLogic) SearchSysUser(req *types.SearchSysUserRequest) (resp *types.SearchSysUserResponse, err error) {

	// 调用 RPC 服务搜索用户
	res, err := l.svcCtx.PortalRpc.UserSearch(l.ctx, &pb.SearchSysUserReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderStr,
		IsAsc:      req.IsAsc,
		Username:   req.Username,
		Nickname:   req.Nickname,
		Email:      req.Email,
		Phone:      req.Phone,
		WorkNumber: req.WorkNumber,
		CreatedBy:  req.CreateBy,
		UpdatedBy:  req.UpdateBy,
	})
	if err != nil {
		l.Errorf("搜索用户失败: error=%v", err)
		return nil, err
	}

	// 转换用户列表
	var users []types.SysUser
	for _, user := range res.Data {
		users = append(users, types.SysUser{
			Id:             user.Id,
			Username:       user.Username,
			Nickname:       user.Nickname,
			Avatar:         user.Avatar,
			Email:          user.Email,
			Phone:          user.Phone,
			WorkNumber:     user.WorkNumber,
			DeptId:         user.DeptId,
			Status:         user.Status,
			IsNeedResetPwd: user.IsNeedResetPwd,
			CreatedBy:      user.CreatedBy,
			UpdatedBy:      user.UpdatedBy,
			CreatedAt:      user.CreatedAt,
			UpdatedAt:      user.UpdatedAt,
		})
	}

	return &types.SearchSysUserResponse{
		Items: users,
		Total: res.Total,
	}, nil
}
