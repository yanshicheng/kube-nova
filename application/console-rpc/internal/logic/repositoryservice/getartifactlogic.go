package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetArtifactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetArtifactLogic {
	return &GetArtifactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetArtifactLogic) GetArtifact(in *pb.GetArtifactReq) (*pb.GetArtifactResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	artifact, err := client.Artifact().Get(in.ProjectName, in.RepoName, in.Reference)
	if err != nil {
		return nil, errorx.Msg("查询制品失败")
	}

	var tags []*pb.Tag
	for _, t := range artifact.Tags {
		tags = append(tags, &pb.Tag{
			Id:        t.ID,
			Name:      t.Name,
			PushTime:  t.PushTime.Unix(),
			PullTime:  t.PullTime.Unix(),
			Immutable: t.Immutable,
			Signed:    t.Signed,
		})
	}

	return &pb.GetArtifactResp{
		Data: &pb.Artifact{
			Id:                artifact.ID,
			Type:              artifact.Type,
			Digest:            artifact.Digest,
			Tags:              tags,
			PushTime:          artifact.PushTime.Unix(),
			PullTime:          artifact.PullTime.Unix(),
			Size:              artifact.Size,
			ManifestMediaType: artifact.ManifestMediaType,
		},
	}, nil
}
