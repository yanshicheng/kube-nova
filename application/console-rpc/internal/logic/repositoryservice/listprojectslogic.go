package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectsLogic {
	return &ListProjectsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 仓库项目操作（管理员操作 + 项目视角查询）============
func (l *ListProjectsLogic) ListProjects(in *pb.ListProjectsReq) (*pb.ListProjectsResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	rq := types.ListRequest{
		Search:   in.Search,
		Page:     in.Page,
		PageSize: in.PageSize,
		SortBy:   in.SortBy,
		SortDesc: in.SortDesc,
	}

	resp, err := client.Project().List(rq)
	if err != nil {
		return nil, errorx.Msg("查询项目列表失败")
	}

	var items []*pb.RegistryProject
	for _, p := range resp.Items {
		items = append(items, &pb.RegistryProject{
			ProjectId:           p.ProjectID,
			Name:                p.Name,
			OwnerName:           p.OwnerName,
			IsPublic:            p.Public,
			RepoCount:           p.RepoCount,
			CreationTime:        p.CreationTime.Unix(),
			UpdateTime:          p.UpdateTime.Unix(),
			StorageLimit:        p.StorageLimit,
			StorageUsed:         p.StorageUsed,
			StorageLimitDisplay: p.StorageLimitDisplay,
			StorageUsedDisplay:  p.StorageUsedDisplay,
		})
	}

	return &pb.ListProjectsResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
