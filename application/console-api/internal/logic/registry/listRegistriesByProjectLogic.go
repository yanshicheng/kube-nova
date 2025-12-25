package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListRegistriesByProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询项目关联的仓库（项目视角）
func NewListRegistriesByProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistriesByProjectLogic {
	return &ListRegistriesByProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRegistriesByProjectLogic) ListRegistriesByProject(req *types.ListRegistriesByProjectRequest) (resp *types.ListRegistriesByProjectResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListRegistriesByProject(l.ctx, &pb.ListRegistriesByProjectReq{
		AppProjectId: req.AppProjectId,
		ClusterUuid:  req.ClusterUuid,
		Page:         req.Page,
		PageSize:     req.PageSize,
		OrderBy:      req.OrderBy,
		IsAsc:        req.IsAsc,
		Name:         req.Name,
		Env:          req.Env,
		Type:         req.Type,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var data []types.ContainerRegistry
	for _, item := range rpcResp.Data {
		data = append(data, types.ContainerRegistry{
			Id:          item.Id,
			Name:        item.Name,
			Uuid:        item.Uuid,
			Type:        item.Type,
			Env:         item.Env,
			Url:         item.Url,
			Username:    item.Username,
			Password:    item.Password,
			Insecure:    item.Insecure,
			CaCert:      item.CaCert,
			Config:      item.Config,
			Status:      item.Status,
			Description: item.Description,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	l.Infof("查询项目关联仓库成功: Total=%d", rpcResp.Total)
	return &types.ListRegistriesByProjectResponse{
		Data:  data,
		Total: rpcResp.Total,
	}, nil
}
