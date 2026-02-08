package pod

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodLogsGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Pod 日志（一次性，不跟踪）
func NewPodLogsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodLogsGetLogic {
	return &PodLogsGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodLogsGetLogic) PodLogsGet(req *types.PodLogsGetReq) (resp *types.PodLogsGetResp, err error) {
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

	// 4. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
	}

	// 5. 构建日志选项
	tailLines := req.TailLines
	if tailLines <= 0 {
		tailLines = 1000 // 默认获取最后 1000 行
	}
	// 限制最大行数，防止内存溢出
	const maxTailLines int64 = 100000
	if tailLines > maxTailLines {
		tailLines = maxTailLines
		l.Infof("TailLines 超过最大限制，已调整为 %d", maxTailLines)
	}

	logOpts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     false, // 一次性获取，不跟踪
		Timestamps: req.Timestamps,
		TailLines:  &tailLines,
		Previous:   req.Previous,
	}

	// 处理时间范围
	if req.SinceSeconds > 0 {
		logOpts.SinceSeconds = &req.SinceSeconds
	} else if req.SinceTime != "" {
		// 解析 RFC3339 格式时间（正确的方式）
		t, err := time.Parse(time.RFC3339, req.SinceTime)
		if err != nil {
			l.Errorf("解析时间失败: %v", err)
			return nil, fmt.Errorf("时间格式错误，请使用 RFC3339 格式")
		}
		sinceTime := metav1.NewTime(t)
		logOpts.SinceTime = &sinceTime
	}

	// 字节数限制
	if req.LimitBytes > 0 {
		logOpts.LimitBytes = &req.LimitBytes
	}

	// 6. 获取日志流
	stream, err := podClient.GetLogs(workloadInfo.Data.Namespace, req.PodName, containerName, logOpts)
	if err != nil {
		l.Errorf("获取日志失败: %v", err)
		return nil, fmt.Errorf("获取日志失败: %v", err)
	}
	defer stream.Close()

	// 7. 读取所有日志
	var logs strings.Builder
	totalLines := 0
	reader := bufio.NewReader(stream)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// 如果最后一行没有换行符，也要添加
				if line != "" {
					logs.WriteString(line)
					totalLines++
				}
				break
			}
			l.Errorf("读取日志失败: %v", err)
			return nil, fmt.Errorf("读取日志失败: %v", err)
		}
		logs.WriteString(line)
		totalLines++
	}

	// 8. 构建响应
	resp = &types.PodLogsGetResp{
		Logs:       logs.String(),
		Container:  containerName,
		TotalLines: totalLines,
	}

	l.Infof("成功获取 Pod [%s] 容器 [%s] 的日志，共 %d 行", req.PodName, containerName, totalLines)

	return resp, nil
}
