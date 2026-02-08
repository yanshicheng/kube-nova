package pod

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodLogsDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	w      http.ResponseWriter
	r      *http.Request
}

// 下载 Pod 日志
func NewPodLogsDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext, w http.ResponseWriter, r *http.Request) *PodLogsDownloadLogic {
	return &PodLogsDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		w:      w,
		r:      r,
	}
}

func (l *PodLogsDownloadLogic) PodLogsDownload(req *types.PodLogsDownloadReq) error {
	// 1. 获取集群和命名空间信息
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

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 如果指定下载所有容器日志
	if req.AllContainers {
		return l.downloadAllContainersLogs(podClient, workloadInfo.Data.Namespace, req)
	}

	// 5. 下载单个容器日志
	return l.downloadSingleContainerLog(podClient, workloadInfo.Data.Namespace, req)
}

// downloadSingleContainerLog 下载单个容器的日志
func (l *PodLogsDownloadLogic) downloadSingleContainerLog(podClient k8stypes.PodOperator, namespace string, req *types.PodLogsDownloadReq) error {
	// 1. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		var err error
		containerName, err = podClient.GetDefaultContainer(namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return fmt.Errorf("获取默认容器失败")
		}
	}

	// 2. 构建日志选项
	logOpts := l.buildLogOptions(req, containerName)

	// 3. 获取日志流
	stream, err := podClient.GetLogs(namespace, req.PodName, containerName, logOpts)
	if err != nil {
		l.Errorf("获取日志失败: %v", err)
		return fmt.Errorf("获取日志失败: %v", err)
	}
	defer stream.Close()

	// 4. 设置响应头
	filename := fmt.Sprintf("%s-%s-%s.log", req.PodName, containerName, time.Now().Format("20060102-150405"))
	l.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	l.w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// 5. 流式传输日志
	written, err := io.Copy(l.w, stream)
	if err != nil {
		l.Errorf("传输日志失败: %v", err)
		return fmt.Errorf("传输日志失败: %v", err)
	}

	l.Infof("成功下载 Pod [%s] 容器 [%s] 的日志，大小: %d 字节", req.PodName, containerName, written)

	return nil
}

// downloadAllContainersLogs 下载所有容器的日志（压缩为一个文件）
func (l *PodLogsDownloadLogic) downloadAllContainersLogs(podClient k8stypes.PodOperator, namespace string, req *types.PodLogsDownloadReq) error {
	// 1. 获取所有容器
	containers, err := podClient.GetAllContainers(namespace, req.PodName)
	if err != nil {
		l.Errorf("获取容器列表失败: %v", err)
		return fmt.Errorf("获取容器列表失败")
	}

	// 2. 设置响应头
	filename := fmt.Sprintf("%s-all-containers-%s.log", req.PodName, time.Now().Format("20060102-150405"))
	l.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	l.w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// 3. 依次下载每个容器的日志
	allContainers := append(containers.Containers, containers.InitContainers...)
	allContainers = append(allContainers, containers.EphemeralContainers...)

	for i, container := range allContainers {
		// 分隔符
		separator := fmt.Sprintf("\n========== Container: %s ==========\n", container.Name)
		l.w.Write([]byte(separator))

		// 构建日志选项
		logOpts := l.buildLogOptions(req, container.Name)

		// 获取日志流
		stream, err := podClient.GetLogs(namespace, req.PodName, container.Name, logOpts)
		if err != nil {
			l.Errorf("获取容器 [%s] 日志失败: %v", container.Name, err)
			l.w.Write([]byte(fmt.Sprintf("Error: %v\n", err)))
			continue
		}

		// 传输日志
		io.Copy(l.w, stream)
		stream.Close()

		// 如果不是最后一个容器，添加换行
		if i < len(allContainers)-1 {
			l.w.Write([]byte("\n"))
		}
	}

	l.Infof("成功下载 Pod [%s] 所有容器的日志", req.PodName)

	return nil
}

// buildLogOptions 构建日志选项
func (l *PodLogsDownloadLogic) buildLogOptions(req *types.PodLogsDownloadReq, containerName string) *corev1.PodLogOptions {
	logOpts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     false,
		Timestamps: req.Timestamps,
		Previous:   req.Previous,
	}

	// 行数限制
	const maxTailLines int64 = 100000
	const defaultTailLines int64 = 10000
	if req.TailLines > 0 {
		tailLines := req.TailLines
		if tailLines > maxTailLines {
			tailLines = maxTailLines
			l.Infof("TailLines 超过最大限制，已调整为 %d", maxTailLines)
		}
		logOpts.TailLines = &tailLines
	} else if req.TailLines < 0 {
		// 负数视为无效，使用默认值
		tailLines := defaultTailLines
		logOpts.TailLines = &tailLines
		l.Infof("TailLines 为负数，使用默认值: %d", defaultTailLines)
	}
	// TailLines == 0 时不��置，由 k8smanager 使用默认值

	// 字节数限制
	if req.MaxSize > 0 {
		logOpts.LimitBytes = &req.MaxSize
	}

	// 时间范围
	if req.StartTime != "" {
		// 使用标准库解析 RFC3339 格式时间
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err == nil {
			startTime := metav1.NewTime(t)
			logOpts.SinceTime = &startTime
		} else {
			l.Errorf("解析开始时间失败: %v", err)
		}
	}

	return logOpts
}
