package pod

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/common/wsutil"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileTailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	ws     *wsutil.WSConnection
}

func NewPodFileTailLogic(ctx context.Context, svcCtx *svc.ServiceContext, ws *wsutil.WSConnection) *PodFileTailLogic {
	return &PodFileTailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		ws:     ws,
	}
}

func (l *PodFileTailLogic) PodFileTail(req *types.PodFileTailReq) error {
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("文件跟踪 panic 恢复: %v", r)
			if !l.ws.IsClosed() {
				l.ws.SendErrorWithCode("PANIC", fmt.Sprintf("服务异常: %v", r))
			}
		}
	}()

	// 1. 获取集群和命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取 Pod 操作器
	podClient := client.Pods()

	// 4. 检查 Pod 状态
	pod, err := podClient.Get(workloadInfo.Data.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取 Pod 失败: %v", err)
		return fmt.Errorf("Pod 不存在或无法访问")
	}

	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
		return fmt.Errorf("Pod 状态异常: %s", pod.Status.Phase)
	}

	// 5. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return fmt.Errorf("获取默认容器失败")
		}
	}

	// 6. 发送初始化成功消息
	if err := l.ws.SendMessage(wsutil.TypeFileTailInit, map[string]interface{}{
		"container": containerName,
		"namespace": workloadInfo.Data.Namespace,
		"podName":   req.PodName,
		"path":      req.Path,
		"message":   "文件跟踪已建立",
	}); err != nil {
		return err
	}

	// 7. 构建 tail 选项
	tailOpts := &k8stypes.TailOptions{
		Lines:  req.Lines,
		Follow: req.Follow,
		Retry:  req.Retry,
	}

	// 8. 开始跟踪文件
	return l.tailFile(workloadInfo.Data.ClusterUuid, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, tailOpts)
}

func (l *PodFileTailLogic) tailFile(clusterUuid, namespace, podName, containerName, path string, tailOpts *k8stypes.TailOptions) error {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		return err
	}
	podClient := client.Pods()

	// 启动文件跟踪
	lineChan, errChan, err := podClient.TailFile(l.ctx, namespace, podName, containerName, path, tailOpts)
	if err != nil {
		l.Errorf("启动文件跟踪失败: %v", err)
		return fmt.Errorf("启动文件跟踪失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(l.ctx)
	defer cancel()

	checkTicker := time.NewTicker(5 * time.Second)
	defer checkTicker.Stop()

	noDataTimer := time.NewTimer(60 * time.Second)
	defer noDataTimer.Stop()

	healthCheckTicker := time.NewTicker(30 * time.Second)
	defer healthCheckTicker.Stop()

	lineNumber := 0

	l.Infof("文件跟踪已启动: namespace=%s, pod=%s, container=%s, path=%s", namespace, podName, containerName, path)

	// 主循环
	for {
		select {
		case <-ctx.Done():
			l.Infof("上下文取消，文件跟踪结束")
			return nil

		case <-l.ws.CloseChan():
			l.Infof("WebSocket 连接关闭，停止文件跟踪")
			return nil

		case line, ok := <-lineChan:
			if !ok {
				l.Infof("文件跟踪通道关闭")
				if !l.ws.IsClosed() {
					l.ws.SendMessage(wsutil.TypeFileTailEnd, map[string]string{
						"message": "文件跟踪已结束",
					})
				}
				return nil
			}

			if !l.ws.IsConnectionAlive() {
				l.Infof("连接不活跃，停止文件跟踪")
				return nil
			}

			// 重置无数据定时器
			if !noDataTimer.Stop() {
				select {
				case <-noDataTimer.C:
				default:
				}
			}
			noDataTimer.Reset(60 * time.Second)

			lineNumber++

			// 发送文件行
			line = strings.TrimSuffix(line, "\n")
			if err := l.ws.SendMessage(wsutil.TypeFileTailData, wsutil.FileTailMessage{
				Line:       line,
				LineNumber: lineNumber,
				Timestamp:  time.Now().UnixMilli(),
			}); err != nil {
				if !l.ws.IsClosed() {
					l.Errorf("发送文件行失败: %v", err)
				}
				return err
			}

		case err, ok := <-errChan:
			if !ok {
				return nil
			}
			l.Errorf("文件跟踪错误: %v", err)
			if !l.ws.IsClosed() {
				l.ws.SendErrorWithCode("TAIL_ERROR", fmt.Sprintf("文件跟踪错误: %v", err))
			}
			return err

		case <-checkTicker.C:
			if !l.ws.IsConnectionAlive() {
				l.Infof("连接健康检查失败，停止文件跟踪")
				return nil
			}
			l.Debugf("连接健康检查通过")

		case <-noDataTimer.C:
			if !l.ws.IsConnectionAlive() {
				l.Infof("60秒无数据且连接不活跃，停止文件跟踪")
				return nil
			}
			if tailOpts.Follow {
				l.Debugf("60秒无新数据但连接活跃，继续等待")
			}
			// 重置定时器继续等待
			noDataTimer.Reset(60 * time.Second)

		case <-healthCheckTicker.C:
			if err := l.checkPodHealth(clusterUuid, namespace, podName, containerName); err != nil {
				l.Errorf("Pod 健康检查失败: %v", err)
				if !l.ws.IsClosed() {
					l.ws.SendErrorWithCode("POD_UNHEALTHY", err.Error())
				}
				return err
			}
		}
	}
}

// checkPodHealth 检查 Pod 健康状态
func (l *PodFileTailLogic) checkPodHealth(clusterUuid, namespace, podName, containerName string) error {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败")
	}

	podClient := client.Pods()
	pod, err := podClient.Get(namespace, podName)
	if err != nil {
		return fmt.Errorf("Pod 不存在或已被删除")
	}

	if pod.DeletionTimestamp != nil {
		return fmt.Errorf("Pod 正在删除中")
	}

	if pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("Pod 已失败")
	}
	if pod.Status.Phase == corev1.PodUnknown {
		return fmt.Errorf("Pod 状态未知")
	}

	// 检查容器状态
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Running != nil {
				return nil
			}
			if cs.State.Terminated != nil {
				return fmt.Errorf("容器已终止: %s", cs.State.Terminated.Reason)
			}
			if cs.State.Waiting != nil {
				return fmt.Errorf("容器正在等待: %s", cs.State.Waiting.Reason)
			}
		}
	}

	return nil
}
