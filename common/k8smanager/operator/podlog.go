package operator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// 日志常量
const (
	DefaultTailLines     = 1000              // 默认尾部行数
	DefaultMaxLogSize    = 10 * 1024 * 1024  // 默认最大日志大小 10MB
	MaxDownloadLogSize   = 100 * 1024 * 1024 // 下载日志最大大小 100MB
	DefaultDownloadLines = 10000             // 下载默认行数
)

// ========== Pod 日志操作实现 ==========

// GetLogs 获取 Pod 日志流
func (p *podOperator) GetLogs(namespace, name, container string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	if namespace == "" || name == "" {
		p.log.Error("获取日志失败：命名空间和Pod名称不能为空")
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	p.log.Infof("获取Pod日志: namespace=%s, pod=%s, container=%s", namespace, name, container)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	// 设置默认日志选项
	if opts == nil {
		opts = &corev1.PodLogOptions{}
		p.log.Debug("使用默认日志选项")
	}

	// 设置容器名称
	opts.Container = container

	if opts.TailLines == nil && !opts.Follow {
		tailLines := int64(DefaultTailLines)
		opts.TailLines = &tailLines
		p.log.Debugf("设置默认尾部行数: %d", tailLines)
	}

	// 处理 TailLines 的特殊值
	if opts.TailLines != nil {
		if *opts.TailLines == 0 {
			p.log.Info("TailLines=0，获取全部日志")
			opts.TailLines = nil
		} else if *opts.TailLines < 0 {
			// 负数视为无效，使用默认值
			tailLines := int64(DefaultTailLines)
			opts.TailLines = &tailLines
			p.log.Infof("TailLines 为负数，使用默认值: %d", tailLines)
		}
	}

	p.log.Infof("最终日志选项: follow=%v, tailLines=%v, timestamps=%v",
		opts.Follow,
		func() string {
			if opts.TailLines != nil {
				return fmt.Sprintf("%d", *opts.TailLines)
			}
			return "nil(全部)"
		}(),
		opts.Timestamps)

	// 获取日志流
	logStream := p.client.CoreV1().
		Pods(namespace).
		GetLogs(name, opts)

	// 执行请求获取日志流
	logs, err := logStream.Stream(p.ctx)
	if err != nil {
		p.log.Errorf("获取Pod日志失败: %s/%s/%s, error=%v", namespace, name, container, err)
		return nil, fmt.Errorf("获取Pod日志失败: %v", err)
	}

	p.log.Infof("成功获取Pod日志流: %s/%s/%s", namespace, name, container)
	return logs, nil
}

// GetLogsWithFollow 获取 Pod 日志并持续跟踪
func (p *podOperator) GetLogsWithFollow(namespace, name, container string, opts *types.LogsOptions) (io.ReadCloser, error) {
	p.log.Infof("获取Pod日志（带跟踪）: namespace=%s, pod=%s, container=%s", namespace, name, container)

	if opts == nil {
		opts = &types.LogsOptions{}
		p.log.Debug("使用默认日志选项")
	}

	// 转换为 Kubernetes API 选项
	podLogOpts := &corev1.PodLogOptions{
		Container:                    opts.Container,
		Follow:                       opts.Follow,
		Previous:                     opts.Previous,
		Timestamps:                   opts.Timestamps,
		InsecureSkipTLSVerifyBackend: opts.InsecureSkipTLSVerifyBackend,
	}

	if opts.SinceSeconds != nil {
		podLogOpts.SinceSeconds = opts.SinceSeconds
		p.log.Debugf("设置时间间隔: %d秒", *opts.SinceSeconds)
	}
	if opts.SinceTime != nil {
		podLogOpts.SinceTime = opts.SinceTime
		p.log.Debugf("设置起始时间点: %v", opts.SinceTime)
	}

	if opts.TailLines != nil {
		if *opts.TailLines > 0 {
			podLogOpts.TailLines = opts.TailLines
			p.log.Debugf("设置尾部行数: %d", *opts.TailLines)
		} else if *opts.TailLines < 0 {
			// 负数视为无效，使用默认值
			tailLines := int64(DefaultTailLines)
			podLogOpts.TailLines = &tailLines
			p.log.Infof("TailLines 为负数，使用默认值: %d", tailLines)
		} else {
			// TailLines=0 表示获取全部日志
			p.log.Debug("TailLines=0，不限制行数")
			podLogOpts.TailLines = nil
		}
	}

	if opts.LimitBytes != nil {
		podLogOpts.LimitBytes = opts.LimitBytes
		p.log.Debugf("设置字节限制: %d", *opts.LimitBytes)
	}

	return p.GetLogs(namespace, name, container, podLogOpts)
}

// GetPreviousLogs 获取容器重启前的日志
func (p *podOperator) GetPreviousLogs(ctx context.Context, namespace, name, container string, opts *types.LogsOptions) (io.ReadCloser, error) {
	p.log.Infof("获取重启前日志: namespace=%s, pod=%s, container=%s", namespace, name, container)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	// 1. 先检查容器是否有重启记录
	pod, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 2. 验证容器重启次数
	hasRestarted := false
	var restartCount int32
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == container {
			restartCount = cs.RestartCount
			if restartCount > 0 {
				hasRestarted = true
				p.log.Infof("容器 %s 已重启 %d 次", container, restartCount)
			}
			break
		}
	}

	// 也检查 Init 容器
	if !hasRestarted {
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.Name == container {
				restartCount = cs.RestartCount
				if restartCount > 0 {
					hasRestarted = true
					p.log.Infof("Init容器 %s 已重启 %d 次", container, restartCount)
				}
				break
			}
		}
	}

	if !hasRestarted {
		p.log.Errorf("容器 %s 没有重启记录，无法获取 Previous 日志", container)
		return nil, fmt.Errorf("容器 %s 未重启过，没有 Previous 日志", container)
	}

	// 3. 获取 Previous 日志
	if opts == nil {
		opts = &types.LogsOptions{}
	}
	opts.Previous = true
	opts.Container = container

	return p.GetLogsWithFollow(namespace, name, container, opts)
}

// DownloadLogs 下载 Pod 日志
func (p *podOperator) DownloadLogs(namespace, name string, opts *types.LogsDownloadOptions) ([]byte, error) {
	// 参数验证
	if namespace == "" || name == "" {
		p.log.Error("下载日志失败：命名空间和Pod名称不能为空")
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	p.log.Infof("开始下载Pod日志: namespace=%s, pod=%s", namespace, name)

	// 初始化选项
	if opts == nil {
		opts = &types.LogsDownloadOptions{
			TailLines:  DefaultDownloadLines,
			Timestamps: true,
			MaxSize:    MaxDownloadLogSize,
		}
		p.log.Debug("使用默认下载选项")
	}

	// 设置默认最大大小
	if opts.MaxSize == 0 {
		opts.MaxSize = MaxDownloadLogSize
	}

	// 创建日志缓冲区
	var allLogs bytes.Buffer

	// 获取 Pod 信息
	p.log.Debug("获取Pod信息以确定容器列表")
	pod, err := p.Get(namespace, name)
	if err != nil {
		p.log.Errorf("获取Pod信息失败: %s/%s, error=%v", namespace, name, err)
		return nil, fmt.Errorf("获取Pod信息失败: %v", err)
	}

	// 确定要收集日志的容器列表
	var targetContainers []string

	if opts.AllContainers {
		p.log.Info("收集所有容器的日志")
		// 按顺序：Init容器 -> 普通容器
		for _, container := range pod.Spec.InitContainers {
			targetContainers = append(targetContainers, container.Name)
		}
		for _, container := range pod.Spec.Containers {
			targetContainers = append(targetContainers, container.Name)
		}
		// 临时容器
		for _, container := range pod.Spec.EphemeralContainers {
			targetContainers = append(targetContainers, container.Name)
		}
		p.log.Infof("找到 %d 个容器", len(targetContainers))
	} else if opts.Container != "" {
		p.log.Infof("收集指定容器的日志: %s", opts.Container)
		targetContainers = []string{opts.Container}
	} else {
		defaultContainer, err := p.GetDefaultContainer(namespace, name)
		if err != nil {
			if len(pod.Spec.Containers) > 0 {
				defaultContainer = pod.Spec.Containers[0].Name
				p.log.Infof("使用第一个容器作为默认容器: %s", defaultContainer)
			} else {
				p.log.Error("Pod中没有找到任何容器")
				return nil, fmt.Errorf("Pod中没有找到任何容器")
			}
		}
		targetContainers = []string{defaultContainer}
	}

	p.log.Infof("准备下载 %d 个容器的日志", len(targetContainers))

	// 遍历每个目标容器，收集日志
	for i, containerName := range targetContainers {
		p.log.Debugf("处理容器 %d/%d: %s", i+1, len(targetContainers), containerName)

		// 添加容器分隔符
		if len(targetContainers) > 1 {
			if i > 0 {
				allLogs.WriteString("\n\n")
			}
			separator := fmt.Sprintf("========== Container: %s ==========\n", containerName)
			separator += fmt.Sprintf("========== Namespace: %s, Pod: %s ==========\n\n", namespace, name)
			allLogs.WriteString(separator)
		}

		// 构建日志获取选项
		podLogOpts := &corev1.PodLogOptions{
			Container:  containerName,
			Previous:   opts.Previous,
			Timestamps: opts.Timestamps,
		}

		// 设置时间范围
		if opts.TimeRange.Start != nil {
			podLogOpts.SinceTime = &metav1.Time{Time: *opts.TimeRange.Start}
			p.log.Debugf("设置日志起始时间: %v", *opts.TimeRange.Start)
		}

		// 设置尾部行数限制
		if opts.TailLines > 0 {
			podLogOpts.TailLines = &opts.TailLines
			p.log.Debugf("限制日志行数: %d", opts.TailLines)
		} else {
			defaultTailLines := int64(DefaultDownloadLines)
			podLogOpts.TailLines = &defaultTailLines
			p.log.Debugf("使用默认行数限制: %d", defaultTailLines)
		}

		// 获取容器日志流
		p.log.Infof("开始获取容器日志: container=%s", containerName)
		logStream, err := p.GetLogs(namespace, name, containerName, podLogOpts)
		if err != nil {
			errMsg := fmt.Sprintf("获取容器 %s 日志失败: %v", containerName, err)
			p.log.Error(errMsg)

			if !opts.ContinueOnError {
				return nil, fmt.Errorf("获取容器日志失败: %v", err)
			}

			allLogs.WriteString(fmt.Sprintf("\n[ERROR] 无法获取容器 %s 的日志: %v\n", containerName, err))
			allLogs.WriteString("[INFO] 跳过此容器，继续处理下一个容器...\n")
			continue
		}

		// 读取日志内容（带大小限制）
		limitReader := io.LimitReader(logStream, opts.MaxSize-int64(allLogs.Len()))
		containerLogs, err := io.ReadAll(limitReader)
		logStream.Close()

		if err != nil {
			errMsg := fmt.Sprintf("读取容器 %s 日志内容失败: %v", containerName, err)
			p.log.Error(errMsg)

			if !opts.ContinueOnError {
				return nil, fmt.Errorf("读取日志内容失败: %v", err)
			}

			allLogs.WriteString(fmt.Sprintf("\n[ERROR] 读取容器 %s 日志时发生错误: %v\n", containerName, err))
			continue
		}

		// 如果设置了结束时间，进行日志过滤
		if opts.TimeRange.End != nil && opts.Timestamps {
			p.log.Infof("根据结束时间过滤日志: %v", opts.TimeRange.End)
			filteredLogs := filterLogsByEndTime(containerLogs, *opts.TimeRange.End)
			containerLogs = filteredLogs
		}

		// 将容器日志添加到总日志缓冲区
		allLogs.Write(containerLogs)
		p.log.Debugf("容器 %s 日志大小: %d bytes", containerName, len(containerLogs))

		// 检查当前日志大小是否超过限制
		if opts.MaxSize > 0 && int64(allLogs.Len()) >= opts.MaxSize {
			p.log.Errorf("日志大小已达到限制 %d 字节，停止收集更多日志", opts.MaxSize)
			break
		}

		p.log.Infof("成功获取容器 %s 的日志，大小: %d 字节", containerName, len(containerLogs))
	}

	// 获取最终的日志数据
	data := allLogs.Bytes()

	// 最终大小检查和截断
	if opts.MaxSize > 0 && int64(len(data)) > opts.MaxSize {
		p.log.Errorf("总日志大小 %d 字节超过限制 %d 字节，进行截断", len(data), opts.MaxSize)
		data = data[:opts.MaxSize]
		truncateMsg := "\n\n[WARNING] 日志已被截断，达到最大大小限制\n"
		if len(data) > len(truncateMsg) {
			copy(data[len(data)-len(truncateMsg):], truncateMsg)
		}
	}

	if len(data) == 0 {
		p.log.Errorf("没有收集到任何日志内容")
		return []byte("没有找到日志内容\n"), nil
	}

	p.log.Infof("日志下载完成，总大小: %d 字节", len(data))
	return data, nil
}

// StreamLogsToWriter 流式传输日志到 Writer（适合 HTTP SSE 或 WebSocket）
func (p *podOperator) StreamLogsToWriter(
	ctx context.Context,
	namespace, name, container string,
	writer io.Writer,
	opts *types.LogsOptions,
) error {

	p.log.Infof("开始流式传输日志: namespace=%s, pod=%s, container=%s", namespace, name, container)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return err
	}

	// 设置 Follow 模式
	if opts == nil {
		opts = &types.LogsOptions{
			Follow: true,
		}
	} else {
		opts.Follow = true
	}

	logStream, err := p.GetLogsWithFollow(namespace, name, container, opts)
	if err != nil {
		return err
	}
	defer logStream.Close()

	scanner := bufio.NewScanner(logStream)
	lineCount := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			p.log.Info("上下文取消，停止日志流")
			return ctx.Err()
		default:
			line := scanner.Text()
			lineCount++

			// 写入一行日志
			if _, err := fmt.Fprintf(writer, "%s\n", line); err != nil {
				p.log.Errorf("写入日志失败: %v", err)
				return fmt.Errorf("写入日志失败: %v", err)
			}

			// 如果 writer 支持 Flush（如 http.ResponseWriter）
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}

			// 定期输出统计
			if lineCount%1000 == 0 {
				p.log.Debugf("已传输 %d 行日志", lineCount)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		p.log.Errorf("扫描日志流错误: %v", err)
		return fmt.Errorf("扫描日志流错误: %v", err)
	}

	p.log.Infof("日志流传输完成，总行数: %d", lineCount)
	return nil
}

// GetLogsMultipleContainers 获取多个容器的日志
func (p *podOperator) GetLogsMultipleContainers(
	ctx context.Context,
	namespace, name string,
	containerNames []string,
	opts *types.LogsOptions,
) (map[string]io.ReadCloser, error) {

	p.log.Infof("获取多个容器的日志: namespace=%s, pod=%s, containers=%v",
		namespace, name, containerNames)

	results := make(map[string]io.ReadCloser)

	for _, containerName := range containerNames {
		p.log.Debugf("获取容器 %s 的日志", containerName)

		logStream, err := p.GetLogsWithFollow(namespace, name, containerName, opts)
		if err != nil {
			p.log.Errorf("获取容器 %s 日志失败: %v", containerName, err)
			// 关闭已打开的流
			for _, stream := range results {
				stream.Close()
			}
			return nil, fmt.Errorf("获取容器 %s 日志失败: %v", containerName, err)
		}

		results[containerName] = logStream
	}

	p.log.Infof("成功获取 %d 个容器的日志流", len(results))
	return results, nil
}

// SearchLogs 搜索日志内容
func (p *podOperator) SearchLogs(
	ctx context.Context,
	namespace, name, container string,
	searchTerm string,
	opts *types.LogsOptions,
) ([]string, error) {

	p.log.Infof("搜索日志: namespace=%s, pod=%s, container=%s, term=%s",
		namespace, name, container, searchTerm)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	logStream, err := p.GetLogsWithFollow(namespace, name, container, opts)
	if err != nil {
		return nil, err
	}
	defer logStream.Close()

	var matchingLines []string
	scanner := bufio.NewScanner(logStream)
	lineCount := 0

	searchLower := strings.ToLower(searchTerm)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			p.log.Info("上下文取消，停止搜索")
			return matchingLines, ctx.Err()
		default:
			line := scanner.Text()
			lineCount++

			if strings.Contains(strings.ToLower(line), searchLower) {
				matchingLines = append(matchingLines, line)
			}

			// 定期输出进度
			if lineCount%1000 == 0 {
				p.log.Debugf("已扫描 %d 行，找到 %d 条匹配", lineCount, len(matchingLines))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		p.log.Errorf("扫描日志时出错: %v", err)
		return matchingLines, fmt.Errorf("扫描日志时出错: %v", err)
	}

	p.log.Infof("搜索完成: 扫描 %d 行，找到 %d 条匹配", lineCount, len(matchingLines))
	return matchingLines, nil
}

// GetLogsStats 获取日志统计信息
func (p *podOperator) GetLogsStats(
	ctx context.Context,
	namespace, name, container string,
	opts *types.LogsOptions,
) (map[string]interface{}, error) {

	p.log.Infof("获取日志统计: namespace=%s, pod=%s, container=%s", namespace, name, container)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	logStream, err := p.GetLogsWithFollow(namespace, name, container, opts)
	if err != nil {
		return nil, err
	}
	defer logStream.Close()

	stats := map[string]interface{}{
		"lineCount":  0,
		"byteCount":  0,
		"errorCount": 0,
		"warnCount":  0,
		"infoCount":  0,
		"debugCount": 0,
		"firstLine":  "",
		"lastLine":   "",
		"hasMore":    false,
	}

	scanner := bufio.NewScanner(logStream)
	lineCount := 0
	byteCount := 0
	var firstLine, lastLine string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			stats["incomplete"] = true
			p.log.Info("上下文取消，统计不完整")
			return stats, ctx.Err()
		default:
			line := scanner.Text()
			lineCount++
			byteCount += len(line) + 1 // +1 for newline

			if firstLine == "" {
				firstLine = line
			}
			lastLine = line

			// 统计日志级别
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "error") {
				stats["errorCount"] = stats["errorCount"].(int) + 1
			} else if strings.Contains(lineLower, "warn") {
				stats["warnCount"] = stats["warnCount"].(int) + 1
			} else if strings.Contains(lineLower, "info") {
				stats["infoCount"] = stats["infoCount"].(int) + 1
			} else if strings.Contains(lineLower, "debug") {
				stats["debugCount"] = stats["debugCount"].(int) + 1
			}
		}
	}

	stats["lineCount"] = lineCount
	stats["byteCount"] = byteCount
	stats["firstLine"] = firstLine
	stats["lastLine"] = lastLine

	if err := scanner.Err(); err != nil {
		p.log.Errorf("扫描日志时出错: %v", err)
		return stats, fmt.Errorf("扫描日志时出错: %v", err)
	}

	p.log.Infof("日志统计完成: 行数=%d, 字节数=%d", lineCount, byteCount)
	return stats, nil
}

// filterLogsByEndTime 根据结束时间过滤日志（改进版本）
func filterLogsByEndTime(logs []byte, endTime time.Time) []byte {
	var filtered bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(logs))

	// 常见的时间戳格式
	timeFormats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
	}

	lineCount := 0
	filteredCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// 尝试解析时间戳
		var logTime time.Time
		parsed := false

		// 提取可能的时间戳部分（前 50 个字符）
		if len(line) > 20 {
			timestampStr := line
			if len(line) > 50 {
				timestampStr = line[:50]
			}

			// 查找时间戳结束位置
			endIdx := strings.IndexAny(timestampStr, " \t")
			if endIdx > 0 {
				timestampStr = timestampStr[:endIdx]

				// 尝试每种格式
				for _, format := range timeFormats {
					if t, err := time.Parse(format, timestampStr); err == nil {
						logTime = t
						parsed = true
						break
					}
				}
			}
		}

		// 如果解析到时间戳且超过结束时间，停止
		if parsed && logTime.After(endTime) {
			break
		}

		// 保留该行
		filtered.WriteString(line)
		filtered.WriteByte('\n')
		filteredCount++
	}

	return filtered.Bytes()
}

// ExportLogsToFile 导出日志到文件（辅助方法）
func (p *podOperator) ExportLogsToFile(
	ctx context.Context,
	namespace, name, container string,
	filePath string,
	opts *types.LogsOptions,
) error {

	p.log.Infof("导出日志到文件: namespace=%s, pod=%s, container=%s, file=%s",
		namespace, name, container, filePath)

	return fmt.Errorf("ExportLogsToFile: 未实现")
}
