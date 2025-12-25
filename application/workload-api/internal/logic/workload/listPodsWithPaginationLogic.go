package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPodsWithPaginationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取Pod列表（带分页、过滤、排序）
func NewListPodsWithPaginationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPodsWithPaginationLogic {
	return &ListPodsWithPaginationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPodsWithPaginationLogic) ListPodsWithPagination(req *types.ListPodsWithPaginationRequest) (resp *types.ListPodsWithPaginationResponse, err error) {
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.Id})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}
	pods, err := client.Pods().List(workspace.Data.Namespace, types2.ListRequest{
		Search:   req.Search,
		Page:     req.Page,
		PageSize: req.PageSize,
		SortBy:   req.SortBy,
		SortDesc: req.SortDesc,
		Labels:   req.Labels,
	})
	resp = &types.ListPodsWithPaginationResponse{
		Total:      pods.Total,
		Page:       pods.Page,
		PageSize:   pods.PageSize,
		TotalPages: pods.TotalPages,
		Items:      make([]types.PodDetailInfo, 0, len(pods.Items)),
	}

	for _, pod := range pods.Items {
		podDetail := types.PodDetailInfo{
			Name:         pod.Name,
			Namespace:    pod.Namespace,
			Status:       pod.Status,
			Ready:        pod.Ready,
			Restarts:     pod.Restarts,
			Age:          pod.Age,
			Node:         pod.Node,
			PodIP:        pod.PodIP,
			Labels:       pod.Labels,
			CreationTime: pod.CreationTime, // 转换为毫秒时间戳
		}
		resp.Items = append(resp.Items, podDetail)
	}

	l.Infof("成功获取 Pod 列表，总数: %d, 当前页: %d/%d", resp.Total, resp.Page, resp.TotalPages)
	return resp, nil
}
