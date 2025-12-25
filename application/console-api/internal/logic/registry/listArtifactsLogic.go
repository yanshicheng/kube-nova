package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListArtifactsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询制品列表
func NewListArtifactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListArtifactsLogic {
	return &ListArtifactsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListArtifactsLogic) ListArtifacts(req *types.ListArtifactsRequest) (resp *types.ListArtifactsResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListArtifacts(l.ctx, &pb.ListArtifactsReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		RepoName:     req.RepoName,
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

	var items []types.Artifact
	for _, item := range rpcResp.Items {
		var tags []types.Tag
		for _, tag := range item.Tags {
			tags = append(tags, types.Tag{
				Id:        tag.Id,
				Name:      tag.Name,
				PushTime:  tag.PushTime,
				PullTime:  tag.PullTime,
				Immutable: tag.Immutable,
				Signed:    tag.Signed,
			})
		}
		items = append(items, types.Artifact{
			Id:                item.Id,
			Type:              item.Type,
			Digest:            item.Digest,
			Tags:              tags,
			PushTime:          item.PushTime,
			PullTime:          item.PullTime,
			Size:              item.Size,
			ManifestMediaType: item.ManifestMediaType,
		})
	}

	l.Infof("查询制品列表成功: Total=%d, Count=%d", rpcResp.Total, len(items))
	return &types.ListArtifactsResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
