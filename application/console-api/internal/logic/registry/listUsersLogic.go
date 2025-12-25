package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出Harbor用户
func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListUsersLogic) ListUsers(req *types.ListUsersRequest) (resp *types.ListUsersResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListUsers(l.ctx, &pb.ListUsersReq{
		RegistryUuid: req.RegistryUuid,
		Search:       req.Search,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.HarborUser
	for _, item := range rpcResp.Items {
		items = append(items, types.HarborUser{
			UserId:          item.UserId,
			Username:        item.Username,
			Email:           item.Email,
			Realname:        item.Realname,
			Comment:         item.Comment,
			CreationTime:    item.CreationTime,
			UpdateTime:      item.UpdateTime,
			SysadminFlag:    item.SysadminFlag,
			AdminRoleInAuth: item.AdminRoleInAuth,
		})
	}

	l.Infof("查询Harbor用户成功: Total=%d", rpcResp.Total)
	return &types.ListUsersResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
