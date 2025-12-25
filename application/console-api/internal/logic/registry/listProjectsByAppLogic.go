package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectsByAppLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询应用项目关联的仓库项目（项目视角）
func NewListProjectsByAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectsByAppLogic {
	return &ListProjectsByAppLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProjectsByAppLogic) ListProjectsByApp(req *types.ListProjectsByAppRequest) (resp *types.ListProjectsByAppResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListProjectsByApp(l.ctx, &pb.ListProjectsByAppReq{
		AppProjectId: req.AppProjectId,
		ClusterUuid:  req.ClusterUuid,
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
			ProjectId:    item.ProjectId,
			Name:         item.Name,
			OwnerName:    item.OwnerName,
			IsPublic:     item.IsPublic,
			RepoCount:    item.RepoCount,
			CreationTime: item.CreationTime,
			UpdateTime:   item.UpdateTime,
		})
	}

	l.Infof("查询应用关联项目成功: Total=%d", rpcResp.Total)
	return &types.ListProjectsByAppResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
