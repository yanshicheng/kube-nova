package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PVCListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PVC 列表
func NewPVCListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCListLogic {
	return &PVCListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCListLogic) PVCList(req *types.ClusterNamespaceRequest) (resp *types.PVCListResponse, err error) {

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	pvcClient := client.PVC()

	pvcList, err := pvcClient.List(req.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 PVC 列表失败: %v", err)
		return nil, fmt.Errorf("获取 PVC 列表失败")
	}

	items := make([]types.PVCListItem, 0, len(pvcList.Items))
	for _, pvc := range pvcList.Items {
		// 转换 AccessModes
		accessModes := make([]string, len(pvc.AccessModes))
		for i, mode := range pvc.AccessModes {
			accessModes[i] = string(mode)
		}

		items = append(items, types.PVCListItem{
			Name:              pvc.Name,
			Namespace:         pvc.Namespace,
			Status:            string(pvc.Status),
			Volume:            pvc.Volume,
			Capacity:          pvc.Capacity,
			AccessModes:       accessModes,
			StorageClass:      pvc.StorageClass,
			Age:               pvc.Age,
			CreationTimestamp: pvc.CreationTimestamp,
		})
	}

	l.Infof("成功获取 PVC 列表，共 %d 个", len(items))
	return &types.PVCListResponse{
		Total: pvcList.Total,
		Items: items,
	}, nil
}
