package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesInRegistryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 仓库内搜索镜像（管理员）
func NewSearchImagesInRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesInRegistryLogic {
	return &SearchImagesInRegistryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchImagesInRegistryLogic) SearchImagesInRegistry(req *types.SearchImagesInRegistryRequest) (resp *types.SearchImagesInRegistryResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.SearchImagesInRegistry(l.ctx, &pb.SearchImagesInRegistryReq{
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

	l.Infof("仓库内搜索成功: Total=%d, Count=%d", rpcResp.Total, len(data))
	return &types.SearchImagesInRegistryResponse{
		Data:  data,
		Total: rpcResp.Total,
	}, nil
}
