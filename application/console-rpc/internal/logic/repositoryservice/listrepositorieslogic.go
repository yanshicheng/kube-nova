package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRepositoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRepositoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRepositoriesLogic {
	return &ListRepositoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ Repository 操作 ============
func (l *ListRepositoriesLogic) ListRepositories(in *pb.ListRepositoriesReq) (*pb.ListRepositoriesResp, error) {
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

	resp, err := client.Repository().List(in.ProjectName, req)
	if err != nil {
		return nil, errorx.Msg("查询仓库列表失败")
	}

	var items []*pb.Repository
	for _, r := range resp.Items {
		items = append(items, &pb.Repository{
			Id:            r.ID,
			Name:          r.Name,
			ProjectId:     r.ProjectID,
			ProjectName:   in.ProjectName,
			Description:   r.Description,
			ArtifactCount: r.ArtifactCount,
			PullCount:     r.PullCount,
			CreationTime:  r.CreationTime.Unix(),
			UpdateTime:    r.UpdateTime.Unix(),
		})
	}

	return &pb.ListRepositoriesResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
