package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListRepositoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询仓库列表
func NewListRepositoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRepositoriesLogic {
	return &ListRepositoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRepositoriesLogic) ListRepositories(req *types.ListRepositoriesRequest) (resp *types.ListRepositoriesResponse, err error) {
	// 调用 RPC 服务查询仓库列表
	rpcResp, err := l.svcCtx.RepositoryRpc.ListRepositories(l.ctx, &pb.ListRepositoriesReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
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

	// 转换 RPC 响应为 API 响应格式
	var items []types.Repository
	for _, item := range rpcResp.Items {
		items = append(items, types.Repository{
			Id:            item.Id,
			Name:          item.Name,
			ProjectId:     item.ProjectId,
			ProjectName:   item.ProjectName,
			Description:   item.Description,
			ArtifactCount: item.ArtifactCount,
			PullCount:     item.PullCount,
			CreationTime:  item.CreationTime,
			UpdateTime:    item.UpdateTime,
		})
	}

	l.Infof("查询仓库列表成功: RegistryUuid=%s, ProjectName=%s, Total=%d, Count=%d",
		req.RegistryUuid, req.ProjectName, rpcResp.Total, len(items))

	return &types.ListRepositoriesResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
