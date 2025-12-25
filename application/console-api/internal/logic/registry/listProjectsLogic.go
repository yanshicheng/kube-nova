package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询仓库所有项目（管理员）
func NewListProjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectsLogic {
	return &ListProjectsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProjectsLogic) ListProjects(req *types.ListProjectsRequest) (resp *types.ListProjectsResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListProjects(l.ctx, &pb.ListProjectsReq{
		RegistryUuid: req.RegistryUuid,
		Search:       req.Search,
		Page:         req.Page,
		PageSize:     req.PageSize,
		SortBy:       req.SortBy,
		SortDesc:     req.SortDesc,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.RegistryProject
	for _, item := range rpcResp.Items {
		items = append(items, types.RegistryProject{
			ProjectId:           item.ProjectId,
			Name:                item.Name,
			OwnerName:           item.OwnerName,
			IsPublic:            item.IsPublic,
			StorageLimitDisplay: item.StorageLimitDisplay,
			StorageUsedDisplay:  item.StorageUsedDisplay,
			StorageUsed:         item.StorageUsed,
			StorageLimit:        item.StorageLimit,
			RepoCount:           item.RepoCount,
			CreationTime:        item.CreationTime,
			UpdateTime:          item.UpdateTime,
		})
	}

	l.Infof("查询仓库项目成功: Total=%d, Count=%d", rpcResp.Total, len(items))
	return &types.ListProjectsResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
