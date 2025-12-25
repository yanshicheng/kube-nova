package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesInRegistryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchImagesInRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesInRegistryLogic {
	return &SearchImagesInRegistryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 3. 仓库内搜索-管理员视角：搜索指定仓库所有项目
func (l *SearchImagesInRegistryLogic) SearchImagesInRegistry(in *pb.SearchImagesInRegistryReq) (*pb.SearchImagesInRegistryResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	images, err := client.SearchImages(in.ImageName)
	if err != nil {
		return nil, errorx.Msg("搜索镜像失败")
	}

	var data []*pb.ImageSearchResult
	for _, img := range images {
		data = append(data, &pb.ImageSearchResult{
			ProjectName:   img.ProjectName,
			RepoName:      img.RepoName,
			Tags:          img.Tags,
			ArtifactCount: 0,
			PullCount:     0,
		})
	}

	return &pb.SearchImagesInRegistryResp{
		Data:  data,
		Total: int64(len(data)),
	}, nil
}
