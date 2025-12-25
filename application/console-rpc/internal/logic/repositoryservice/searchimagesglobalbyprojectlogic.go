package repositoryservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesGlobalByProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchImagesGlobalByProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesGlobalByProjectLogic {
	return &SearchImagesGlobalByProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 2. 全局搜索-项目视角：搜索项目关联的仓库和有权限的项目
func (l *SearchImagesGlobalByProjectLogic) SearchImagesGlobalByProject(in *pb.SearchImagesGlobalByProjectReq) (*pb.SearchImagesGlobalByProjectResp, error) {
	// 1. 查询该应用项目在指定集群下关联的所有仓库
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

	// 2. 获取所有关联的仓库
	var registryIds []uint64
	for _, c := range clusters {
		registryIds = append(registryIds, c.RegistryId)
	}

	if len(registryIds) == 0 {
		return &pb.SearchImagesGlobalByProjectResp{Data: []*pb.RegistryImageSearchResult{}, Total: 0}, nil
	}

	// 3. 查询每个仓库的绑定项目
	registryProjectMap := make(map[uint64]map[string]bool) // registry_id -> project_name -> bool
	for _, clusterId := range clusters {
		bindings, _ := l.svcCtx.RegistryProjectBindingModel.SearchNoPage(
			l.ctx,
			"",
			true,
			"`registry_cluster_id` = ? AND `app_project_id` = ?",
			clusterId.Id, in.AppProjectId,
		)

		if _, ok := registryProjectMap[clusterId.RegistryId]; !ok {
			registryProjectMap[clusterId.RegistryId] = make(map[string]bool)
		}

		for _, b := range bindings {
			registryProjectMap[clusterId.RegistryId][b.RegistryProjectName] = true
		}
	}

	// 4. 遍历每个仓库搜索镜像
	var results []*pb.RegistryImageSearchResult
	for _, registryId := range registryIds {
		reg, err := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, registryId)
		if err != nil {
			continue
		}

		// 获取客户端
		client, err := l.svcCtx.HarborManager.Get(reg.Uuid)
		if err != nil {
			continue
		}

		// 搜索镜像
		allImages, err := client.SearchImages(in.ImageName)
		if err != nil {
			continue
		}

		// 过滤：只返回公开项目 + 绑定的私有项目
		var filteredImages []*pb.ImageSearchResult
		for _, img := range allImages {
			// 检查项目是否公开或已绑定
			project, _ := client.Project().Get(img.ProjectName)
			if project != nil && (project.Public || registryProjectMap[registryId][img.ProjectName]) {
				filteredImages = append(filteredImages, &pb.ImageSearchResult{
					ProjectName:   img.ProjectName,
					RepoName:      img.RepoName,
					Tags:          img.Tags,
					ArtifactCount: 0,
					PullCount:     0,
				})
			}
		}

		if len(filteredImages) > 0 {
			results = append(results, &pb.RegistryImageSearchResult{
				RegistryName: reg.Name,
				RegistryUuid: reg.Uuid,
				RegistryUrl:  reg.Url,
				Images:       filteredImages,
			})
		}
	}

	return &pb.SearchImagesGlobalByProjectResp{
		Data:  results,
		Total: int64(len(results)),
	}, nil
}
