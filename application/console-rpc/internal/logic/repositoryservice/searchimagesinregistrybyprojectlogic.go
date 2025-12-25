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

type SearchImagesInRegistryByProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchImagesInRegistryByProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesInRegistryByProjectLogic {
	return &SearchImagesInRegistryByProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 4. 仓库内搜索-项目视角：搜索指定仓库中项目关联的项目
func (l *SearchImagesInRegistryByProjectLogic) SearchImagesInRegistryByProject(in *pb.SearchImagesInRegistryByProjectReq) (*pb.SearchImagesInRegistryByProjectResp, error) {
	// 1. 查找 registry_cluster_id
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

	// 3. 获取客户端并搜索
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	allImages, err := client.SearchImages(in.ImageName)
	if err != nil {
		return nil, errorx.Msg("搜索镜像失败")
	}

	// 4. 过滤：只返回公开项目 + 绑定的私有项目
	var data []*pb.ImageSearchResult
	for _, img := range allImages {
		project, _ := client.Project().Get(img.ProjectName)
		if project != nil && (project.Public || privateProjects[img.ProjectName]) {
			data = append(data, &pb.ImageSearchResult{
				ProjectName:   img.ProjectName,
				RepoName:      img.RepoName,
				Tags:          img.Tags,
				ArtifactCount: 0,
				PullCount:     0,
			})
		}
	}

	return &pb.SearchImagesInRegistryByProjectResp{
		Data:  data,
		Total: int64(len(data)),
	}, nil
}
