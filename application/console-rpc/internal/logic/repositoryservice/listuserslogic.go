package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListUsers 列出所有 Harbor 用户
func (l *ListUsersLogic) ListUsers(in *pb.ListUsersReq) (*pb.ListUsersResp, error) {
	// 获取 Harbor 客户端
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 构建请求
	req := types.ListRequest{
		Search:   in.Search,
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	// 获取用户列表
	resp, err := client.User().List(req)
	if err != nil {
		return nil, errorx.Msg("查询用户列表失败")
	}

	// 转换数据
	var items []*pb.HarborUser
	for _, u := range resp.Items {
		items = append(items, &pb.HarborUser{
			UserId:          u.UserID,
			Username:        u.Username,
			Email:           u.Email,
			Realname:        u.Realname,
			Comment:         u.Comment,
			CreationTime:    u.CreationTime.Unix(),
			UpdateTime:      u.UpdateTime.Unix(),
			SysadminFlag:    u.SysAdminFlag,
			AdminRoleInAuth: u.AdminRoleInAuth,
		})
	}

	return &pb.ListUsersResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
