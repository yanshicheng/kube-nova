package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/client/repositoryservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesGlobalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 全局搜索镜像（管理员）
func NewSearchImagesGlobalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesGlobalLogic {
	return &SearchImagesGlobalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchImagesGlobalLogic) SearchImagesGlobal(req *types.SearchImagesGlobalRequest) (resp *types.SearchImagesGlobalResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.SearchImagesGlobal(l.ctx, &repositoryservice.SearchImagesGlobalReq{
		ImageName:    req.ImageName,
		RegistryType: req.RegistryType,
		Env:          req.Env,
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

	l.Infof("全局搜索镜像成功: Total=%d, RegistryCount=%d", rpcResp.Total, len(data))
	return &types.SearchImagesGlobalResponse{
		Data:  data,
		Total: rpcResp.Total,
	}, nil
}
