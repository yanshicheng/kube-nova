package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetArtifactLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取制品详情
func NewGetArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetArtifactLogic {
	return &GetArtifactLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetArtifactLogic) GetArtifact(req *types.GetArtifactRequest) (resp *types.GetArtifactResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetArtifact(l.ctx, &pb.GetArtifactReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		RepoName:     req.RepoName,
		Reference:    req.Reference,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	var tags []types.Tag
	for _, tag := range data.Tags {
		tags = append(tags, types.Tag{
			Id:        tag.Id,
			Name:      tag.Name,
			PushTime:  tag.PushTime,
			PullTime:  tag.PullTime,
			Immutable: tag.Immutable,
			Signed:    tag.Signed,
		})
	}

	l.Infof("获取制品详情成功: Digest=%s, TagCount=%d", data.Digest, len(tags))
	return &types.GetArtifactResponse{
		Data: types.Artifact{
			Id:                data.Id,
			Type:              data.Type,
			Digest:            data.Digest,
			Tags:              tags,
			PushTime:          data.PushTime,
			PullTime:          data.PullTime,
			Size:              data.Size,
			ManifestMediaType: data.ManifestMediaType,
		},
	}, nil
}
