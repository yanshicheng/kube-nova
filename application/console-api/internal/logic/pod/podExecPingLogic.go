package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	corev1 "k8s.io/api/core/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodExecPingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Exec 会话心跳检查
func NewPodExecPingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodExecPingLogic {
	return &PodExecPingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodExecPingLogic) PodExecPing(req *types.PodExecPingReq) (resp *types.PodExecPingResp, err error) {
	// 1. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 检查 Pod 是否存在
	pod, err := podClient.Get(workloadInfo.Data.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取 Pod 失败: %v", err)
		return &types.PodExecPingResp{
			Status:          "dead",
			ContainerStatus: "not_found",
		}, nil
	}

	// 5. 检查 Pod 是否正在运行
	if pod.Status.Phase != corev1.PodRunning {
		return &types.PodExecPingResp{
			Status:          "dead",
			ContainerStatus: string(pod.Status.Phase),
		}, nil
	}

	// 6. 如果指定了容器，检查容器状态
	containerStatus := "running"
	if req.Container != "" {
		// 查找指定容器的状态
		found := false
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == req.Container {
				found = true
				if !cs.Ready {
					containerStatus = "not_ready"
				}
				if cs.State.Waiting != nil {
					containerStatus = fmt.Sprintf("waiting:%s", cs.State.Waiting.Reason)
				} else if cs.State.Terminated != nil {
					containerStatus = fmt.Sprintf("terminated:%s", cs.State.Terminated.Reason)
				}
				break
			}
		}
		if !found {
			containerStatus = "not_found"
		}
	}

	// 7. 构建响应
	status := "alive"
	if containerStatus != "running" {
		status = "dead"
	}

	resp = &types.PodExecPingResp{
		Status:          status,
		ContainerStatus: containerStatus,
	}

	l.Infof("心跳检查完成，Pod: %s, 状态: %s, 容器状态: %s", req.PodName, status, containerStatus)

	return resp, nil
}
