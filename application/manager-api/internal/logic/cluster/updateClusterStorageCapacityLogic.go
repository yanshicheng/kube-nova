package cluster

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateClusterStorageCapacityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateClusterStorageCapacityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateClusterStorageCapacityLogic {
	return &UpdateClusterStorageCapacityLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateClusterStorageCapacityLogic) UpdateClusterStorageCapacity(req *types.UpdateClusterStorageCapacityRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.UpdateClusterStorageCapacity(l.ctx, &managerservice.UpdateClusterStorageCapacityReq{
		Id:                      req.ResourceId,
		StoragePhysicalCapacity: req.StoragePhysicalCapacity,
	})
	if err != nil {
		l.Errorf("集群存储更新失败: %s", err.Error())
		return "", err
	}
	return "集群存储更新成功", nil
}
