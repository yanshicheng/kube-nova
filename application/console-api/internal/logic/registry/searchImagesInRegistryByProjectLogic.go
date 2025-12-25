package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesInRegistryByProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 仓库内搜索镜像（项目视角）
func NewSearchImagesInRegistryByProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesInRegistryByProjectLogic {
	return &SearchImagesInRegistryByProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchImagesInRegistryByProjectLogic) SearchImagesInRegistryByProject(req *types.SearchImagesInRegistryByProjectRequest) (resp *types.SearchImagesInRegistryByProjectResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.SearchImagesInRegistryByProject(l.ctx, &pb.SearchImagesInRegistryByProjectReq{
		AppProjectId: req.AppProjectId,
		ClusterUuid:  req.ClusterUuid,
		RegistryUuid: req.RegistryUuid,
		ImageName:    req.ImageName,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var data []types.ImageSearchResult
	for _, img := range rpcResp.Data {
		data = append(data, types.ImageSearchResult{
			ProjectName:   img.ProjectName,
			RepoName:      img.RepoName,
			Tags:          img.Tags,
			ArtifactCount: img.ArtifactCount,
			PullCount:     img.PullCount,
		})
	}

	l.Infof("项目视角仓库内搜索成功: Total=%d", rpcResp.Total)
	return &types.SearchImagesInRegistryByProjectResponse{
		Data:  data,
		Total: rpcResp.Total,
	}, nil
}
