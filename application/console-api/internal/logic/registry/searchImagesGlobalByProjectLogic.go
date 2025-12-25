package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesGlobalByProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 全局搜索镜像（项目视角）
func NewSearchImagesGlobalByProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesGlobalByProjectLogic {
	return &SearchImagesGlobalByProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchImagesGlobalByProjectLogic) SearchImagesGlobalByProject(req *types.SearchImagesGlobalByProjectRequest) (resp *types.SearchImagesGlobalByProjectResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.SearchImagesGlobalByProject(l.ctx, &pb.SearchImagesGlobalByProjectReq{
		AppProjectId: req.AppProjectId,
		ClusterUuid:  req.ClusterUuid,
		ImageName:    req.ImageName,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var data []types.RegistryImageSearchResult
	for _, regResult := range rpcResp.Data {
		var images []types.ImageSearchResult
		for _, img := range regResult.Images {
			images = append(images, types.ImageSearchResult{
				ProjectName:   img.ProjectName,
				RepoName:      img.RepoName,
				Tags:          img.Tags,
				ArtifactCount: img.ArtifactCount,
				PullCount:     img.PullCount,
			})
		}
		data = append(data, types.RegistryImageSearchResult{
			RegistryName: regResult.RegistryName,
			RegistryUuid: regResult.RegistryUuid,
			RegistryUrl:  regResult.RegistryUrl,
			Images:       images,
		})
	}

	l.Infof("项目视角全局搜索成功: Total=%d", rpcResp.Total)
	return &types.SearchImagesGlobalByProjectResponse{
		Data:  data,
		Total: rpcResp.Total,
	}, nil
}
