package canary

import (
	"context"
	"fmt"
	"strings"
	"time"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryListApplicationLevelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取canary应用服务级别
func NewCanaryListApplicationLevelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryListApplicationLevelLogic {
	return &CanaryListApplicationLevelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryListApplicationLevelLogic) CanaryListApplicationLevel(req *types.CanaryApplicationListRequest) (resp []types.CanaryListItem, err error) {
	// 获取工作空间信息
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		l.Errorf("获取工作空间失败: %v", err)
		return nil, fmt.Errorf("获取工作空间失败: %v", err)
	}
	// 查询所有的应用 的所有版本
	versions, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: req.ApplicationId,
	})
	if err != nil {
		l.Errorf("获取应用版本失败: %v", err)
		return nil, fmt.Errorf("获取应用版本失败: %v", err)
	}
	// 查询服务
	application, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: req.ApplicationId,
	})
	if len(versions.Data) == 0 {
		return []types.CanaryListItem{}, nil
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}
	var flaggerList []*flaggerv1.Canary
	switch strings.ToLower(application.Data.ResourceType) {
	case "deployment":
		for _, version := range versions.Data {
			deplyment, err := client.Deployment().Get(workspace.Data.Namespace, version.ResourceName)
			if err != nil {
				l.Errorf("跳过版本 %s 的 Deployment: %v", version.ResourceName, err)
				continue
			}
			// 通过ref查询关联的 flagger
			flagger, err := client.Flagger().GetByTargetRef(workspace.Data.Namespace, deplyment.APIVersion, deplyment.Kind, version.ResourceName)
			if err != nil {
				l.Infof("版本 %s 没有关联的 Flagger 资源，跳过", version.ResourceName)
				continue
			}
			if flagger == nil {
				l.Infof("版本 %s 的 Flagger 资源为空，跳过", version.ResourceName)
				continue
			}
			flaggerList = append(flaggerList, flagger)
		}
	case "statefulset":
		for _, version := range versions.Data {
			statefulset, err := client.StatefulSet().Get(workspace.Data.Namespace, version.ResourceName)
			if err != nil {
				l.Errorf("获取 StatefulSet 失败: %v", err)
				l.Errorf("跳过版本 %s 的 StatefulSet: %v", version.ResourceName, err)
				continue
			}
			// 通过ref查询关联的 flagger
			flagger, err := client.Flagger().GetByTargetRef(workspace.Data.Namespace, statefulset.APIVersion, statefulset.Kind, statefulset.Name)
			if err != nil {
				l.Infof("版本 %s 没有关联的 Flagger 资源，跳过", version.ResourceName)
				continue
			}
			if flagger == nil {
				l.Infof("版本 %s 的 Flagger 资源为空，跳过", version.ResourceName)
				continue
			}
			flaggerList = append(flaggerList, flagger)
		}
	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", application.Data.ResourceType)
	}
	// 把flaggerList 转为 resp []*types.CanaryListResponse
	for _, canary := range flaggerList {
		if canary == nil {
			l.Errorf("跳过 nil Canary 对象")
			continue
		}
		item := l.convertCanaryToListItem(canary)
		resp = append(resp, item)
	}

	return
}

// convertCanaryToListItem 将 Canary 对象转换为 CanaryListItem
func (l *CanaryListApplicationLevelLogic) convertCanaryToListItem(canary *flaggerv1.Canary) types.CanaryListItem {
	if canary == nil {
		l.Error("convertCanaryToListItem: canary 对象为 nil")
		return types.CanaryListItem{}
	}

	if canary.Spec.TargetRef.Kind == "" || canary.Spec.TargetRef.Name == "" {
		l.Errorf("Canary %s/%s 的 TargetRef 信息不完整", canary.Namespace, canary.Name)
	}

	item := types.CanaryListItem{
		Name:      canary.Name,
		Namespace: canary.Namespace,
		TargetRef: types.TargetRefInfo{
			Kind:       canary.Spec.TargetRef.Kind,
			Name:       canary.Spec.TargetRef.Name,
			ApiVersion: canary.Spec.TargetRef.APIVersion,
		},
		Labels:      canary.Labels,
		Annotations: canary.Annotations,
	}

	// 设置进度截止时间
	if canary.Spec.ProgressDeadlineSeconds != nil {
		item.ProgressDeadline = *canary.Spec.ProgressDeadlineSeconds
	}

	// 设置状态信息
	if canary.Status.Phase != "" {
		item.Phase = string(canary.Status.Phase)
	}
	item.CanaryWeight = canary.Status.CanaryWeight
	item.FailedChecks = canary.Status.FailedChecks
	//item.Iterations = canary.Status.Iterations

	// 设置状态字符串（根据 Phase 映射到 Status）
	item.Status = l.mapPhaseToStatus(canary.Status.Phase)

	if !canary.Status.LastTransitionTime.IsZero() {
		item.LastTransition = canary.Status.LastTransitionTime.Format(time.RFC3339)
	} else {
		item.LastTransition = ""
	}

	// 计算年龄
	item.Age = l.calculateAge(canary.CreationTimestamp)
	item.CreationTimestamp = canary.CreationTimestamp.UnixMilli()

	return item
}

// mapPhaseToStatus 将 Phase 映射到 Status
func (l *CanaryListApplicationLevelLogic) mapPhaseToStatus(phase flaggerv1.CanaryPhase) string {
	switch phase {
	case flaggerv1.CanaryPhaseInitializing:
		return "Initialized"
	case flaggerv1.CanaryPhaseProgressing:
		return "Progressing"
	case flaggerv1.CanaryPhasePromoting:
		return "Promoting"
	case flaggerv1.CanaryPhaseFinalising:
		return "Finalising"
	case flaggerv1.CanaryPhaseSucceeded:
		return "Succeeded"
	case flaggerv1.CanaryPhaseFailed:
		return "Failed"
	default:
		return string(phase)
	}
}

// calculateAge 计算资源的年龄
func (l *CanaryListApplicationLevelLogic) calculateAge(creationTime metav1.Time) string {
	duration := time.Since(creationTime.Time)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}
