package repositoryservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectsByAppLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectsByAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectsByAppLogic {
	return &ListProjectsByAppLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 项目视角：查询应用项目关联的仓库项目（私有项目+公开项目）
func (l *ListProjectsByAppLogic) ListProjectsByApp(in *pb.ListProjectsByAppReq) (*pb.ListProjectsByAppResp, error) {
	// 1. 查询该应用项目在指定集群和仓库下绑定的私有项目
	clusters, err := l.svcCtx.RepositoryClusterModel.SearchNoPage(
		l.ctx,
		"",
		true,
		"`cluster_uuid` = ?",
		in.ClusterUuid,
	)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, errorx.Msg("查询集群仓库关联失败")
	}

	// 找到对应仓库的 cluster ID
	var registryClusterId uint64
	for _, c := range clusters {
		reg, _ := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, c.RegistryId)
		if reg != nil && reg.Uuid == in.RegistryUuid {
			registryClusterId = c.Id
			break
		}
	}

	if registryClusterId == 0 {
		return nil, errorx.Msg("该集群未绑定此仓库")
	}

	// 2. 查询绑定的私有项目
	bindings, _ := l.svcCtx.RegistryProjectBindingModel.SearchNoPage(
		l.ctx,
		"",
		true,
		"`registry_id` = ? AND `app_project_id` = ?",
		registryClusterId, in.AppProjectId,
	)

	privateProjects := make(map[string]bool)
	for _, b := range bindings {
		privateProjects[b.RegistryProjectName] = true
	}

	// 3. 从仓库获取所有项目
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := types.ListRequest{
		Search:   in.Search,
		Page:     in.Page,
		PageSize: in.PageSize,
		SortBy:   in.SortBy,
		SortDesc: in.SortDesc,
	}

	resp, err := client.Project().List(req)
	if err != nil {
		return nil, errorx.Msg("查询项目列表失败")
	}

	// 4. 过滤：只返回公开项目 + 绑定的私有项目
	var items []*pb.RegistryProject
	for _, p := range resp.Items {
		if p.Public || privateProjects[p.Name] {
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
	}

	return &pb.ListProjectsByAppResp{
		Items:      items,
		Total:      int64(len(items)),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64((len(items) + int(in.PageSize) - 1) / int(in.PageSize)),
	}, nil
}
