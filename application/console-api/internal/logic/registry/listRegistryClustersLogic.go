package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListRegistryClustersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询仓库的集群列表
func NewListRegistryClustersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistryClustersLogic {
	return &ListRegistryClustersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRegistryClustersLogic) ListRegistryClusters(req *types.ListRegistryClustersRequest) (resp *types.ListRegistryClustersResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListRegistryClusters(l.ctx, &pb.ListRegistryClustersReq{
		RegistryId: req.RegistryId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var data []types.RegistryCluster
	for _, item := range rpcResp.Data {
		rc := types.RegistryCluster{
			Id:          item.Id,
			RegistryId:  item.RegistryId,
			ClusterUuid: item.ClusterUuid,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
		if item.Registry != nil {
			reg := &types.ContainerRegistry{
				Id:          item.Registry.Id,
				Name:        item.Registry.Name,
				Uuid:        item.Registry.Uuid,
				Type:        item.Registry.Type,
				Env:         item.Registry.Env,
				Url:         item.Registry.Url,
				Username:    item.Registry.Username,
				Password:    item.Registry.Password,
				Insecure:    item.Registry.Insecure,
				CaCert:      item.Registry.CaCert,
				Config:      item.Registry.Config,
				Status:      item.Registry.Status,
				Description: item.Registry.Description,
				CreatedBy:   item.Registry.CreatedBy,
				UpdatedBy:   item.Registry.UpdatedBy,
				CreatedAt:   item.Registry.CreatedAt,
				UpdatedAt:   item.Registry.UpdatedAt,
			}
			rc.Registry = reg
		}
		data = append(data, rc)
	}

	l.Infof("查询仓库集群列表成功: Count=%d", len(data))
	return &types.ListRegistryClustersResponse{
		Data: data,
	}, nil
}
