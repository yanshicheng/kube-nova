package pod

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/common/wsutil"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
)

type PodLogsStreamLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	ws     *wsutil.WSConnection
}

func NewPodLogsStreamLogic(ctx context.Context, svcCtx *svc.ServiceContext, ws *wsutil.WSConnection) *PodLogsStreamLogic {
	return &PodLogsStreamLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		ws:     ws,
	}
}

func (l *PodLogsStreamLogic) PodLogsStream(req *types.PodLogsStreamReq) error {
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("日志流 panic 恢复: %v", r)
			if !l.ws.IsClosed() && !l.ws.IsClientClosed() {
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

	// 4. 检查 Pod 是否存在且运行中
	pod, err := podClient.Get(workloadInfo.Data.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取 Pod 失败: %v", err)
		return fmt.Errorf("Pod 不存在或无法访问")
	}

	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
		return fmt.Errorf("Pod 状态异常: %s，无法获取日志", pod.Status.Phase)
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

	// 6. 验证容器是否存在且就绪
	if err := l.checkContainerReady(pod, containerName); err != nil {
		return err
	}

	var tailLinesPtr *int64
	const maxTailLines int64 = 100000
	if req.TailLines > 0 {
		// 用户指定了行数，限制最大值
		tailLines := req.TailLines
		if tailLines > maxTailLines {
			tailLines = maxTailLines
			l.Infof("[日志流] TailLines 超过最大限制，已调整为 %d", maxTailLines)
		}
		tailLinesPtr = &tailLines
		l.Infof("[日志流] 设置尾部行数: %d", tailLines)
	} else if req.TailLines < 0 {
		// 负数视为无效，使用默认值
		defaultTailLines := int64(100)
		tailLinesPtr = &defaultTailLines
		l.Infof("[日志流] TailLines 为负数，使用默认值: %d", defaultTailLines)
	} else {
		// TailLines = 0，获取全部日志
		l.Info("[日志流] TailLines=0，获取全部日志")
		tailLinesPtr = nil
	}

	// 构建日志选项
	logOpts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		Timestamps: req.Timestamps,
		TailLines:  tailLinesPtr,
		Previous:   req.Previous,
	}

	if req.SinceSeconds > 0 {
		logOpts.SinceSeconds = &req.SinceSeconds
		l.Infof("[日志流] 设置时间范围: 最近 %d 秒", req.SinceSeconds)
	}

	l.Infof("[日志流] 最终参数: namespace=%s, pod=%s, container=%s, follow=%v, timestamps=%v, tailLines=%v, sinceSeconds=%v",
		workloadInfo.Data.Namespace, req.PodName, containerName,
		logOpts.Follow, logOpts.Timestamps,
		func() string {
			if logOpts.TailLines != nil {
				return fmt.Sprintf("%d", *logOpts.TailLines)
			}
			return "nil(全部)"
		}(),
		func() string {
			if logOpts.SinceSeconds != nil {
				return fmt.Sprintf("%d", *logOpts.SinceSeconds)
			}
			return "nil"
		}())

	// 8. 发送初始化成功消息
	if err := l.ws.SendMessage(wsutil.TypeLogInit, map[string]interface{}{
		"container": containerName,
		"namespace": workloadInfo.Data.Namespace,
		"podName":   req.PodName,
		"message":   "日志流已建立",
	}); err != nil {
		l.Errorf("发送初始化消息失败: %v", err)
		return err
	}

	// 9. 启动日志流
	return l.streamLogs(workloadInfo.Data.ClusterUuid, workloadInfo.Data.Namespace, req.PodName, containerName, logOpts)
}

// streamLogs 流式读取并发送日志
func (l *PodLogsStreamLogic) streamLogs(clusterUuid, namespace, podName, containerName string, logOpts *corev1.PodLogOptions) error {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		return err
	}
	podClient := client.Pods()

	// 获取日志流
	stream, err := podClient.GetLogs(namespace, podName, containerName, logOpts)
	if err != nil {
		l.Errorf("获取日志流失败: %v", err)
		return fmt.Errorf("获取日志流失败: %v", err)
	}
	defer stream.Close()

	// 创建上下文
	ctx, cancel := context.WithCancel(l.ctx)
	defer cancel()

	checkTicker := time.NewTicker(3 * time.Second)
	defer checkTicker.Stop()

	// 无日志超时检测（30 秒）
	noLogTimer := time.NewTimer(30 * time.Second)
	defer noLogTimer.Stop()

	// 健康检查（降低频率到 30 秒）
	healthCheckTicker := time.NewTicker(30 * time.Second)
	defer healthCheckTicker.Stop()

	// 读取日志的通道
	lineChan := make(chan string, 100)
	errChan := make(chan error, 1)

	// 启动读取协程
	go func() {
		defer close(lineChan)
		reader := bufio.NewReaderSize(stream, 64*1024) // 64KB buffer

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					select {
					case errChan <- err:
					default:
					}
				}
				return
			}

			select {
			case lineChan <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	l.Infof("[日志流] 已启动: namespace=%s, pod=%s, container=%s", namespace, podName, containerName)

	lineCount := 0

	// 主循环
	for {
		select {
		case <-ctx.Done():
			l.Infof("[日志流] 上下文取消，日志流结束，共发送 %d 行", lineCount)
			return nil

		case <-l.ws.CloseChan():
			l.Infof("[日志流] WebSocket 连接关闭，停止日志流，共发送 %d 行", lineCount)
			return nil

		case line, ok := <-lineChan:
			if !ok {
				l.Infof("[日志流] 日志流正常结束，共发送 %d 行", lineCount)
				if !l.ws.IsClosed() && !l.ws.IsClientClosed() {
					l.ws.SendMessage(wsutil.TypeLogEnd, map[string]interface{}{
						"message":   "日志流已结束",
						"lineCount": lineCount,
					})
				}
				return nil
			}

			if l.ws.IsClosed() || l.ws.IsClientClosed() {
				l.Infof("[日志流] 连接已关闭，停止日志流，共发送 %d 行", lineCount)
				return nil
			}

			// 重置无日志定时器
			if !noLogTimer.Stop() {
				select {
				case <-noLogTimer.C:
				default:
				}
			}
			noLogTimer.Reset(30 * time.Second)

			// 解析并发送日志
			line = strings.TrimSuffix(line, "\n")
			timestamp := ""
			if logOpts.Timestamps {
				parts := strings.SplitN(line, " ", 2)
				if len(parts) == 2 {
					timestamp = parts[0]
					line = parts[1]
				}
			}

			if err := l.ws.SendLog(line, "stdout", timestamp, containerName); err != nil {
				if !l.ws.IsClientClosed() {
					l.Errorf("[日志流] 发送日志失败: %v", err)
				}
				return err
			}

			lineCount++

			if lineCount%1000 == 0 {
				l.Debugf("[日志流] 已发送 %d 行日志", lineCount)
			}

		case err := <-errChan:
			l.Errorf("[日志流] 读取日志失败: %v", err)
			if !l.ws.IsClosed() && !l.ws.IsClientClosed() {
				l.ws.SendErrorWithCode("LOG_READ_ERROR", fmt.Sprintf("读取日志失败: %v", err))
			}
			return err

		case <-checkTicker.C:
			if l.ws.IsClosed() || l.ws.IsClientClosed() {
				l.Infof("[日志流] 连接健康检查：连接已关闭，停止日志流")
				return nil
			}
			if !l.ws.IsConnectionAlive() {
				l.Infof("[日志流] 连接健康检查失败，停止日志流")
				return nil
			}
			l.Debugf("[日志流] 连接健康检查通过")

		case <-noLogTimer.C:
			// 30 秒无日志时检查连接
			if l.ws.IsClosed() || l.ws.IsClientClosed() {
				l.Infof("[日志流] 30秒无日志且连接已关闭，停止日志流")
				return nil
			}
			if !l.ws.IsConnectionAlive() {
				l.Infof("[日志流] 30秒无日志且连接不活跃，停止日志流")
				return nil
			}
			l.Debugf("[日志流] 30秒无日志但连接活跃，继续等待")
			// 重置定时器继续等待
			noLogTimer.Reset(30 * time.Second)

		case <-healthCheckTicker.C:
			// 降低健康检查频率到 30 秒
			if l.ws.IsClosed() || l.ws.IsClientClosed() {
				l.Infof("[日志流] Pod 健康检查：连接已关闭，停止日志流")
				return nil
			}
			if err := l.checkPodHealth(clusterUuid, namespace, podName, containerName); err != nil {
				l.Errorf("[日志流] Pod 健康检查失败: %v", err)
				if !l.ws.IsClosed() && !l.ws.IsClientClosed() {
					l.ws.SendErrorWithCode("POD_UNHEALTHY", err.Error())
				}
				return err
			}
		}
	}
}

// checkContainerReady 检查容器是否就绪
func (l *PodLogsStreamLogic) checkContainerReady(pod *corev1.Pod, containerName string) error {
	// 查找容器状态
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Running != nil {
				return nil
			}
			if cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0 {
				return nil
			}
			if cs.State.Waiting != nil {
				return fmt.Errorf("容器 %s 正在等待: %s", containerName, cs.State.Waiting.Reason)
			}
			if cs.State.Terminated != nil {
				return fmt.Errorf("容器 %s 已终止: %s (退出码: %d)",
					containerName, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
			}
		}
	}

	// 检查 Init 容器
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Running != nil || (cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0) {
				return nil
			}
			return fmt.Errorf("Init 容器 %s 未就绪", containerName)
		}
	}

	return fmt.Errorf("容器 %s 不存在", containerName)
}

// checkPodHealth 检查 Pod 健康状态
func (l *PodLogsStreamLogic) checkPodHealth(clusterUuid, namespace, podName, containerName string) error {
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

	return l.checkContainerReady(pod, containerName)
}
