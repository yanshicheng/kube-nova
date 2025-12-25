package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectMembersLogic {
	return &ListProjectMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 用户管理（项目成员管理）============
func (l *ListProjectMembersLogic) ListProjectMembers(in *pb.ListProjectMembersReq) (*pb.ListProjectMembersResp, error) {
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

	// 获取成员列表
	resp, err := client.Member().List(in.ProjectName, req)
	if err != nil {
		return nil, errorx.Msg("查询项目成员失败")
	}

	// 转换数据
	var items []*pb.ProjectMember
	for _, m := range resp.Items {
		items = append(items, &pb.ProjectMember{
			Id:           m.ID,
			ProjectId:    m.ProjectID,
			EntityName:   m.EntityName,
			EntityType:   m.EntityType,
			RoleId:       m.RoleID,
			RoleName:     m.RoleName,
			CreationTime: m.CreationTime.Unix(),
			UpdateTime:   m.UpdateTime.Unix(),
		})
	}

	return &pb.ListProjectMembersResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
