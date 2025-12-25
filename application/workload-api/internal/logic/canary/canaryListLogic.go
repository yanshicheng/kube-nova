package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 列表
func NewCanaryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryListLogic {
	return &CanaryListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryListLogic) CanaryList(req *types.CanaryListRequest) (resp *types.CanaryListResponse, err error) {
	if req == nil {
		l.Error("CanaryList: 请求参数为 nil")
		return nil, fmt.Errorf("请求参数不能为空")
	}

	// 获取工作空间信息
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		l.Errorf("获取工作空间失败: %v", err)
		return nil, fmt.Errorf("获取工作空间失败: %v", err)
	}

	if workspace == nil || workspace.Data == nil {
		l.Error("工作空间响应数据为空")
		return nil, fmt.Errorf("工作空间数据为空")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}

	if client == nil {
		l.Error("集群客户端为空")
		return nil, fmt.Errorf("集群客户端为空")
	}

	canaryOperator := client.Flagger()
	if canaryOperator == nil {
		l.Error("Flagger 操作器为空")
		return nil, fmt.Errorf("Flagger 操作器未初始化")
	}

	canaryList, err := canaryOperator.List(workspace.Data.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 Canary 列表失败: %v", err)
		return nil, fmt.Errorf("获取 Canary 列表失败: %v", err)
	}

	if canaryList == nil {
		l.Errorf("Canary 列表为空，返回空结果")
		return &types.CanaryListResponse{
			Total: 0,
			Items: []types.CanaryListItem{},
		}, nil
	}

	// 构造响应
	resp = &types.CanaryListResponse{
		Total: canaryList.Total,
		Items: make([]types.CanaryListItem, 0, len(canaryList.Items)),
	}

	// 转换 CanaryInfo 到 CanaryListItem
	for _, item := range canaryList.Items {
		if item.Name == "" {
			l.Errorf("跳过名称为空的 Canary 项")
			continue
		}

		if item.TargetRef.Kind == "" && item.TargetRef.Name == "" {
			l.Errorf("Canary %s/%s 的 TargetRef 信息为空", item.Namespace, item.Name)
		}

		listItem := types.CanaryListItem{
			Name:      item.Name,
			Namespace: item.Namespace,
			TargetRef: types.TargetRefInfo{
				Kind:       item.TargetRef.Kind,
				Name:       item.TargetRef.Name,
				ApiVersion: item.TargetRef.APIVersion,
			},
			ProgressDeadline:  item.ProgressDeadline,
			Status:            item.Status,
			CanaryWeight:      item.CanaryWeight,
			FailedChecks:      item.FailedChecks,
			Phase:             item.Phase,
			LastTransition:    item.LastTransition,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
			Age:               item.Age,
			CreationTimestamp: item.CreationTimestamp,
		}
		resp.Items = append(resp.Items, listItem)
	}

	return resp, nil
}
