package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetRegistryDetailsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取仓库详情
func NewGetRegistryDetailsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRegistryDetailsLogic {
	return &GetRegistryDetailsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRegistryDetailsLogic) GetRegistryDetails(req *types.GetRegistryDetailsRequest) (resp *types.GetRegistryDetailsResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetRegistryDetails(l.ctx, &pb.GetRegistryDetailsReq{
		Uuid: req.Uuid,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("获取仓库详情成功: UUID=%s, TotalProjects=%d", req.Uuid, data.TotalProjects)

	return &types.GetRegistryDetailsResponse{
		Data: types.ContainerRegistryDetails{
			Id:                  data.Id,
			Name:                data.Name,
			Uuid:                data.Uuid,
			Type:                data.Type,
			Env:                 data.Env,
			Url:                 data.Url,
			Username:            data.Username,
			Password:            data.Password,
			Insecure:            data.Insecure,
			CaCert:              data.CaCert,
			Config:              data.Config,
			Status:              data.Status,
			Description:         data.Description,
			CreatedBy:           data.CreatedBy,
			UpdatedBy:           data.UpdatedBy,
			CreatedAt:           data.CreatedAt,
			UpdatedAt:           data.UpdatedAt,
			StorageTotal:        data.StorageTotal,
			StorageUsed:         data.StorageUsed,
			StorageFree:         data.StorageFree,
			TotalProjects:       data.TotalProjects,
			PublicProjects:      data.PublicProjects,
			PrivateProjects:     data.PrivateProjects,
			TotalRepositories:   data.TotalRepositories,
			PublicRepositories:  data.PublicRepositories,
			PrivateRepositories: data.PrivateRepositories,
		},
	}, nil

	return
}
