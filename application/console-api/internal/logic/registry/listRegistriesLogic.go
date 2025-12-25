package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListRegistriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询所有仓库（管理员）
func NewListRegistriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistriesLogic {
	return &ListRegistriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRegistriesLogic) ListRegistries(req *types.ListRegistriesRequest) (resp *types.ListRegistriesResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListRegistries(l.ctx, &pb.ListRegistriesReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		OrderBy:  req.OrderBy,
		IsAsc:    req.IsAsc,
		Name:     req.Name,
		Uuid:     req.Uuid,
		Env:      req.Env,
		Type:     req.Type,
		Status:   req.Status,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var data []types.ContainerRegistry
	for _, item := range rpcResp.Data {
		data = append(data, types.ContainerRegistry{
			Id:                item.Id,
			Name:              item.Name,
			Uuid:              item.Uuid,
			Type:              item.Type,
			Env:               item.Env,
			Url:               item.Url,
			Username:          item.Username,
			Password:          item.Password,
			Insecure:          item.Insecure,
			CaCert:            item.CaCert,
			Config:            item.Config,
			Status:            item.Status,
			Description:       item.Description,
			CreatedBy:         item.CreatedBy,
			UpdatedBy:         item.UpdatedBy,
			CreatedAt:         item.CreatedAt,
			UpdatedAt:         item.UpdatedAt,
			TotalRepositories: item.TotalRepositories,
			TotalProjects:     item.TotalProjects,
			StorageTotal:      item.StorageTotal,
		})
	}

	l.Infof("查询仓库列表成功: Total=%d, Count=%d", rpcResp.Total, len(data))
	return &types.ListRegistriesResponse{
		Data:  data,
		Total: rpcResp.Total,
	}, nil
}
