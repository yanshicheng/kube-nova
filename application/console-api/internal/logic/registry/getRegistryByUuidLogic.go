package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetRegistryByUuidLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据UUID查询仓库
func NewGetRegistryByUuidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRegistryByUuidLogic {
	return &GetRegistryByUuidLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRegistryByUuidLogic) GetRegistryByUuid(req *types.GetRegistryByUuidRequest) (resp *types.GetRegistryByUuidResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetRegistryByUuid(l.ctx, &pb.GetRegistryByUuidReq{
		Uuid: req.Uuid,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("查询镜像仓库成功: UUID=%s, Name=%s", req.Uuid, data.Name)

	return &types.GetRegistryByUuidResponse{
		Data: types.ContainerRegistry{
			Id:          data.Id,
			Name:        data.Name,
			Uuid:        data.Uuid,
			Type:        data.Type,
			Env:         data.Env,
			Url:         data.Url,
			Username:    data.Username,
			Password:    data.Password,
			Insecure:    data.Insecure,
			CaCert:      data.CaCert,
			Config:      data.Config,
			Status:      data.Status,
			Description: data.Description,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt,
			UpdatedAt:   data.UpdatedAt,
		},
	}, nil
}
