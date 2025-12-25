package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListArtifactsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListArtifactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListArtifactsLogic {
	return &ListArtifactsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 制品/标签操作 ============
func (l *ListArtifactsLogic) ListArtifacts(in *pb.ListArtifactsReq) (*pb.ListArtifactsResp, error) {
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

	resp, err := client.Artifact().List(in.ProjectName, in.RepoName, req)
	if err != nil {
		return nil, errorx.Msg("查询制品列表失败")
	}

	var items []*pb.Artifact
	for _, a := range resp.Items {
		var tags []*pb.Tag
		for _, t := range a.Tags {
			tags = append(tags, &pb.Tag{
				Id:        t.ID,
				Name:      t.Name,
				PushTime:  t.PushTime.Unix(),
				PullTime:  t.PullTime.Unix(),
				Immutable: t.Immutable,
				Signed:    t.Signed,
			})
		}

		items = append(items, &pb.Artifact{
			Id:                a.ID,
			Type:              a.Type,
			Digest:            a.Digest,
			Tags:              tags,
			PushTime:          a.PushTime.Unix(),
			PullTime:          a.PullTime.Unix(),
			Size:              a.Size,
			ManifestMediaType: a.ManifestMediaType,
		})
	}

	return &pb.ListArtifactsResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
