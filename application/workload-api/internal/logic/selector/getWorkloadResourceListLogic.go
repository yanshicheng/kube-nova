package selector

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWorkloadResourceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工作负载资源列表
func NewGetWorkloadResourceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWorkloadResourceListLogic {
	return &GetWorkloadResourceListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetWorkloadResourceListLogic) GetWorkloadResourceList(req *types.WorkloadResourceListRequest) (resp *types.WorkloadResourceListResponse, err error) {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	var items []types.WorkloadNameItem

	// 根据 type 调用对应的 operator
	switch req.Type {
	case "deployment":
		deploymentOp := client.Deployment()
		deployList, err := deploymentOp.ListAll(req.Namespace)
		if err != nil {
			return nil, fmt.Errorf("获取 Deployment 列表失败: %v", err)
		}
		items = make([]types.WorkloadNameItem, len(deployList))
		for i, d := range deployList {
			items[i] = types.WorkloadNameItem{
				Name:      d.Name,
				Namespace: d.Namespace,
				Labels:    d.Labels,
				CreatedAt: d.CreationTimestamp.UnixMilli(),
			}
		}

	case "statefulset":
		stsList, err := client.StatefulSet().ListAll(req.Namespace)
		if err != nil {
			return nil, fmt.Errorf("获取 StatefulSet 列表失败: %v", err)
		}
		items = make([]types.WorkloadNameItem, len(stsList))
		for i, s := range stsList {
			items[i] = types.WorkloadNameItem{
				Name:      s.Name,
				Namespace: s.Namespace,
				Labels:    s.Labels,
				CreatedAt: s.CreationTimestamp.UnixMilli(),
			}
		}

	case "daemonset":
		dsList, err := client.DaemonSet().ListAll(req.Namespace)
		if err != nil {
			return nil, fmt.Errorf("获取 DaemonSet 列表失败: %v", err)
		}
		items = make([]types.WorkloadNameItem, len(dsList))
		for i, d := range dsList {
			items[i] = types.WorkloadNameItem{
				Name:      d.Name,
				Namespace: d.Namespace,
				Labels:    d.Labels,
				CreatedAt: d.CreationTimestamp.UnixMilli(),
			}
		}

	case "cronjob":
		cronJobList, err := client.CronJob().ListAll(req.Namespace)
		if err != nil {
			return nil, fmt.Errorf("获取 CronJob 列表失败: %v", err)
		}
		items = make([]types.WorkloadNameItem, len(cronJobList))
		for i, c := range cronJobList {
			items[i] = types.WorkloadNameItem{
				Name:      c.Name,
				Namespace: c.Namespace,
				Labels:    c.Labels,
				CreatedAt: c.CreationTimestamp.UnixMilli(),
			}
		}

	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", req.Type)
	}

	return &types.WorkloadResourceListResponse{
		Total: len(items),
		Items: items,
	}, nil
}
