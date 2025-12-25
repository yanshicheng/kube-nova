package repositoryservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchImagesGlobalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchImagesGlobalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchImagesGlobalLogic {
	return &SearchImagesGlobalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 镜像搜索（4个接口）============
func (l *SearchImagesGlobalLogic) SearchImagesGlobal(in *pb.SearchImagesGlobalReq) (*pb.SearchImagesGlobalResp, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if in.RegistryType != "" {
		conditions = append(conditions, "`type` = ?")
		args = append(args, in.RegistryType)
	}
	if in.Env != "" {
		conditions = append(conditions, "`env` = ?")
		args = append(args, in.Env)
	}
	conditions = append(conditions, "`status` = ?")
	args = append(args, 1) // 只查询启用的仓库

	queryStr := ""
	if len(conditions) > 0 {
		queryStr = strings.Join(conditions, " AND ")
	}

	// 查询所有符合条件的仓库
	registries, err := l.svcCtx.ContainerRegistryModel.SearchNoPage(l.ctx, "", true, queryStr, args...)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, errorx.Msg("查询仓库列表失败")
	}

	var results []*pb.RegistryImageSearchResult
	for _, reg := range registries {
		// 获取客户端
		client, err := l.svcCtx.HarborManager.Get(reg.Uuid)
		if err != nil {
			continue
		}

		// 搜索镜像
		images, err := client.SearchImages(in.ImageName)
		if err != nil {
			continue
		}

		if len(images) > 0 {
			var pbImages []*pb.ImageSearchResult
			for _, img := range images {
				pbImages = append(pbImages, &pb.ImageSearchResult{
					ProjectName:   img.ProjectName,
					RepoName:      img.RepoName,
					Tags:          img.Tags,
					ArtifactCount: 0,
					PullCount:     0,
				})
			}

			results = append(results, &pb.RegistryImageSearchResult{
				RegistryName: reg.Name,
				RegistryUuid: reg.Uuid,
				RegistryUrl:  reg.Url,
				Images:       pbImages,
			})
		}
	}

	return &pb.SearchImagesGlobalResp{
		Data:  results,
		Total: int64(len(results)),
	}, nil
}
