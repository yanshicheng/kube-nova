package pod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/common/wsutil"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodExecLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	ws     *wsutil.WSConnection
}

const (
	shellProbeTimeout  = 3 * time.Second
	shellSelectTimeout = 12 * time.Second
	shellCacheTTL      = 10 * time.Minute
)

type shellCacheEntry struct {
	shell    []string
	expireAt time.Time
}

var (
	shellCache   = make(map[string]shellCacheEntry)
	shellCacheMu sync.RWMutex
)

func NewPodExecLogic(ctx context.Context, svcCtx *svc.ServiceContext, ws *wsutil.WSConnection) *PodExecLogic {
	return &PodExecLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		ws:     ws,
	}
}

func (l *PodExecLogic) reflectExec(podClient interface{}, namespace, podName, containerName string, command []string, opts k8stypes.ExecOptions) (remotecommand.Executor, error) {
	val := reflect.ValueOf(podClient)
	method := val.MethodByName("Exec")

	if !method.IsValid() {
		return nil, fmt.Errorf("客户端 %T 没有 Exec 方法", podClient)
	}

	args := []reflect.Value{
		reflect.ValueOf(namespace),
		reflect.ValueOf(podName),
		reflect.ValueOf(containerName),
		reflect.ValueOf(command),
		reflect.ValueOf(opts),
	}

	results := method.Call(args)

	if len(results) != 2 {
		return nil, fmt.Errorf("Exec 返回值数量异常")
	}

	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	execObj := results[0].Interface()
	executor, ok := execObj.(remotecommand.Executor)
	if !ok {
		return nil, fmt.Errorf("返回对象未实现 remotecommand.Executor: %T", execObj)
	}

	return executor, nil
}

func (l *PodExecLogic) probeShell(ctx context.Context, podClient interface{}, namespace, podName, containerName string, shell []string) error {
	// 1. 构造探测命令
	probeCmd := make([]string, len(shell))
	copy(probeCmd, shell)
	if len(shell) > 0 {
		cmdName := shell[0]
		if strings.Contains(cmdName, "sh") || strings.Contains(cmdName, "bash") {
			probeCmd = append(probeCmd, "-c", "exit 0")
		}
	}

	execOpts := k8stypes.ExecOptions{
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
		Container: containerName,
		Command:   probeCmd,
	}

	executor, err := l.reflectExec(podClient, namespace, podName, containerName, probeCmd, execOpts)
	if err != nil {
		return err
	}

	probeCtx, cancel := context.WithTimeout(ctx, shellProbeTimeout)
	defer cancel()

	var stderrBuf bytes.Buffer

	err = executor.StreamWithContext(probeCtx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: &stderrBuf,
		Tty:    false,
	})

	// 4. 分析错误
	if err != nil {
		fullErrMsg := fmt.Sprintf("%v, stderr: %s", err, stderrBuf.String())
		return fmt.Errorf("%s", fullErrMsg)
	}

	return nil
}

func (l *PodExecLogic) shellCacheKey(clusterUuid, namespace, podName, containerName string) string {
	return strings.Join([]string{clusterUuid, namespace, podName, containerName}, "|")
}

func copyShellCommand(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func (l *PodExecLogic) getCachedShell(clusterUuid, namespace, podName, containerName string) ([]string, bool) {
	key := l.shellCacheKey(clusterUuid, namespace, podName, containerName)

	shellCacheMu.RLock()
	entry, ok := shellCache[key]
	shellCacheMu.RUnlock()
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expireAt) {
		shellCacheMu.Lock()
		delete(shellCache, key)
		shellCacheMu.Unlock()
		return nil, false
	}

	return copyShellCommand(entry.shell), true
}

func (l *PodExecLogic) setCachedShell(clusterUuid, namespace, podName, containerName string, shell []string) {
	key := l.shellCacheKey(clusterUuid, namespace, podName, containerName)
	shellCacheMu.Lock()
	shellCache[key] = shellCacheEntry{
		shell:    copyShellCommand(shell),
		expireAt: time.Now().Add(shellCacheTTL),
	}
	shellCacheMu.Unlock()
}

func (l *PodExecLogic) clearCachedShell(clusterUuid, namespace, podName, containerName string) {
	key := l.shellCacheKey(clusterUuid, namespace, podName, containerName)
	shellCacheMu.Lock()
	delete(shellCache, key)
	shellCacheMu.Unlock()
}

func (l *PodExecLogic) selectAvailableShell(podClient interface{}, clusterUuid, namespace, podName, containerName string, requestedCommand []string) ([]string, string, error) {
	if cachedShell, ok := l.getCachedShell(clusterUuid, namespace, podName, containerName); ok {
		if err := l.probeShell(l.ctx, podClient, namespace, podName, containerName, cachedShell); err == nil {
			cachedShellStr := strings.Join(cachedShell, " ")
			msg := fmt.Sprintf("已连接到终端 (%s)", cachedShellStr)
			if len(requestedCommand) > 0 && strings.Join(requestedCommand, " ") != cachedShellStr {
				msg = fmt.Sprintf("命令 '%s' 不可用，自动切换至 %s",
					strings.Join(requestedCommand, " "), cachedShellStr)
			}
			return cachedShell, msg, nil
		}
		l.clearCachedShell(clusterUuid, namespace, podName, containerName)
	}

	candidates := l.buildShellCandidates(requestedCommand)

	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("没有可用的 Shell 候选")
	}

	l.Infof("开始顺序探测 %d 个 Shell 候选...", len(candidates))

	ctx, cancel := context.WithTimeout(l.ctx, shellSelectTimeout)
	defer cancel()

	var lastErr error
	failedShells := make([]string, 0)

	// 顺序探测，避免并发探测在网络抖动时放大超时和限流问题
	for _, shell := range candidates {
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}

		shellStr := strings.Join(shell, " ")
		l.Infof("探测: %s", shellStr)

		err := l.probeShell(ctx, podClient, namespace, podName, containerName, shell)
		if err == nil {
			l.Infof("Shell 可用: %s", shellStr)
			l.setCachedShell(clusterUuid, namespace, podName, containerName, shell)
			msg := fmt.Sprintf("已连接到终端 (%s)", shellStr)
			if len(requestedCommand) > 0 && strings.Join(requestedCommand, " ") != shellStr {
				msg = fmt.Sprintf("命令 '%s' 不可用，自动切换至 %s",
					strings.Join(requestedCommand, " "), shellStr)
			}
			return shell, msg, nil
		}

		l.Infof("Shell 不可用: %s, 原因: %v", shellStr, err)
		failedShells = append(failedShells, shellStr)
		lastErr = err
	}

	if len(failedShells) > 0 {
		// 优先保留上下文超时之外的最后错误，便于上层分类
		if ctx.Err() != nil && lastErr == nil {
			lastErr = ctx.Err()
		}
	}

	// 返回失败
	return nil, "", fmt.Errorf("未找到可用 Shell (已尝试: %v), 最后错误: %v",
		failedShells, lastErr)
}

func (l *PodExecLogic) buildShellCandidates(requestedCommand []string) [][]string {
	seen := make(map[string]bool)
	candidates := make([][]string, 0, 6)

	// 辅助函数：添加候选（去重）
	addCandidate := func(shell []string) {
		if len(shell) == 0 {
			return
		}
		shellStr := strings.Join(shell, " ")
		if !seen[shellStr] {
			seen[shellStr] = true
			candidates = append(candidates, shell)
		}
	}

	// 1. 用户指定的命令（最高优先级）
	if len(requestedCommand) > 0 {
		addCandidate(requestedCommand)
	}

	// 2. 常用 shell（按优先级）
	addCandidate([]string{"/bin/bash"})
	addCandidate([]string{"/bin/sh"})
	addCandidate([]string{"/bin/ash"})
	addCandidate([]string{"bash"})
	addCandidate([]string{"sh"})

	return candidates
}

func classifyShellProbeError(err error) (code, message string) {
	if err == nil {
		return "", ""
	}

	errText := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errText, "forbidden"):
		return "EXEC_FORBIDDEN", "无法启动终端: 当前服务账号缺少 pods/exec 权限"
	case strings.Contains(errText, "context deadline exceeded"):
		return "EXEC_TIMEOUT", "无法启动终端: 连接容器终端超时，请稍后重试"
	case strings.Contains(errText, "dial tcp"), strings.Contains(errText, "no such host"),
		(strings.Contains(errText, "lookup") && strings.Contains(errText, "timeout")),
		strings.Contains(errText, "tls handshake timeout"), strings.Contains(errText, "i/o timeout"):
		return "CLUSTER_ERROR", "无法启动终端: 集群 API 连接异常，请检查网络或 DNS"
	case strings.Contains(errText, "rate limiter wait returned an error"):
		return "EXEC_TIMEOUT", "无法启动终端: 集群请求拥塞，请稍后重试"
	case strings.Contains(errText, "container not found"), strings.Contains(errText, "not found in pod"):
		return "CONTAINER_ERROR", "无法启动终端: 目标容器不存在或已重建，请刷新后重试"
	default:
		return "", ""
	}
}

// messageDispatcher 消息分发器
type messageDispatcher struct {
	ws         *wsutil.WSConnection
	stdinChan  chan string
	resizeChan chan *resizeMessage
	closeChan  chan struct{}
	closeOnce  sync.Once
	logger     logx.Logger
}

type resizeMessage struct {
	Rows uint16
	Cols uint16
}

func newMessageDispatcher(ws *wsutil.WSConnection, logger logx.Logger) *messageDispatcher {
	return &messageDispatcher{
		ws:         ws,
		stdinChan:  make(chan string, 100),
		resizeChan: make(chan *resizeMessage, 10),
		closeChan:  make(chan struct{}),
		logger:     logger,
	}
}

func (d *messageDispatcher) start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.logger.Errorf("消息分发器 panic 恢复: %v", r)
			}
			d.closeOnce.Do(func() {
				close(d.closeChan)
				close(d.stdinChan)
				close(d.resizeChan)
			})
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-d.closeChan:
				return
			case <-d.ws.CloseChan():
				return
			default:
				d.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
				var msg wsutil.WSMessage
				if err := d.ws.ReadJSON(&msg); err != nil {
					if err != io.EOF && !d.ws.IsClosed() {
						d.logger.Errorf("WebSocket 读取失败: %v", err)
					}
					return
				}
				switch msg.Type {
				case wsutil.TypeExecStdin:
					d.handleStdin(msg, ctx)
				case wsutil.TypeExecResize:
					d.handleResize(msg, ctx)
				}
			}
		}
	}()
}

func (d *messageDispatcher) handleStdin(msg wsutil.WSMessage, ctx context.Context) {
	var input wsutil.ExecStdinMessage
	data, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(data, &input); err != nil {
		return
	}
	select {
	case d.stdinChan <- input.Data:
	case <-ctx.Done():
	case <-d.closeChan:
	case <-time.After(3 * time.Second):
	}
}

func (d *messageDispatcher) handleResize(msg wsutil.WSMessage, ctx context.Context) {
	var resize wsutil.ExecResizeMessage
	data, _ := json.Marshal(msg.Data)
	if err := json.Unmarshal(data, &resize); err != nil {
		return
	}
	select {
	case d.resizeChan <- &resizeMessage{Rows: resize.Rows, Cols: resize.Cols}:
	case <-ctx.Done():
	case <-d.closeChan:
	case <-time.After(3 * time.Second):
	}
}

func (d *messageDispatcher) close() {
	d.closeOnce.Do(func() {
		close(d.closeChan)
	})
}

// xtermReader 实现 io.Reader
type xtermReader struct {
	dispatcher *messageDispatcher
	buffer     []byte
}

func (r *xtermReader) Read(p []byte) (int, error) {
	if len(r.buffer) > 0 {
		n := copy(p, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	}
	select {
	case input, ok := <-r.dispatcher.stdinChan:
		if !ok {
			return 0, io.EOF
		}
		r.buffer = []byte(input)
		n := copy(p, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	case <-r.dispatcher.closeChan:
		return 0, io.EOF
	}
}

// xtermWriter 实现 io.Writer
type xtermWriter struct {
	ws     *wsutil.WSConnection
	stream string
}

func (w *xtermWriter) Write(p []byte) (int, error) {
	if w.ws.IsClosed() || !w.ws.IsConnectionAlive() {
		return 0, io.ErrClosedPipe
	}
	if err := w.ws.SendExecOutput(string(p), w.stream); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (l *PodExecLogic) PodExec(req *types.PodExecReq) error {
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("Exec panic: %v", r)
			if !l.ws.IsClosed() {
				l.ws.SendErrorWithCode("INTERNAL_ERROR", "服务内部异常")
			}
		}
	}()

	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		return l.sendWsError("WORKSPACE_ERROR", "无法获取工作空间信息")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		return l.sendWsError("CLUSTER_ERROR", "无法连接到集群")
	}

	podClient := client.Pods()

	pod, err := podClient.Get(workloadInfo.Data.Namespace, req.PodName)
	if err != nil {
		return l.sendWsError("POD_NOT_FOUND", "Pod 不存在")
	}

	if pod.Status.Phase != corev1.PodRunning {
		return l.sendWsError("POD_NOT_RUNNING", fmt.Sprintf("Pod 状态异常: %s", pod.Status.Phase))
	}

	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			return l.sendWsError("CONTAINER_ERROR", "无法获取默认容器")
		}
	}

	selectedShell, initMessage, err := l.selectAvailableShell(
		podClient,
		workloadInfo.Data.ClusterUuid,
		workloadInfo.Data.Namespace,
		req.PodName,
		containerName,
		req.Command,
	)
	if err != nil {
		l.Errorf("Shell 探测完全失败: %v", err)
		if code, msg := classifyShellProbeError(err); code != "" {
			return l.sendWsError(code, msg)
		}
		return l.sendWsError("NO_SHELL", "无法启动终端: 容器内未找到可用 Shell (sh/bash)")
	}

	// 建立最终的交互式会话
	execOpts := k8stypes.ExecOptions{
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
		Container: containerName,
		Command:   selectedShell,
	}

	executor, err := l.reflectExec(podClient, workloadInfo.Data.Namespace, req.PodName, containerName, selectedShell, execOpts)
	if err != nil {
		l.Errorf("创建会话执行器失败: %v", err)
		return l.sendWsError("CONNECT_FAILED", "建立终端会话失败")
	}

	l.ws.SendMessage(wsutil.TypeExecInit, map[string]interface{}{
		"container": containerName,
		"namespace": workloadInfo.Data.Namespace,
		"podName":   req.PodName,
		"command":   selectedShell,
		"message":   initMessage,
	})

	ctx, cancel := context.WithCancel(l.ctx)
	defer cancel()

	dispatcher := newMessageDispatcher(l.ws, l.Logger)
	dispatcher.start(ctx)
	defer dispatcher.close()

	streams := k8stypes.IOStreams{
		In:     &xtermReader{dispatcher: dispatcher},
		Out:    &xtermWriter{ws: l.ws, stream: "stdout"},
		ErrOut: &xtermWriter{ws: l.ws, stream: "stderr"},
	}

	sizeQueue := newDynamicTerminalSizeQueue(uint16(req.Rows), uint16(req.Cols))
	defer sizeQueue.close()

	go l.handleResizeMessages(ctx, dispatcher.resizeChan, sizeQueue)
	go l.healthCheck(ctx, workloadInfo.Data.ClusterUuid, workloadInfo.Data.Namespace, req.PodName, containerName)
	go l.connectionCheck(ctx)

	l.Infof("开始 Stream 会话: %v", selectedShell)

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             streams.In,
		Stdout:            streams.Out,
		Stderr:            streams.ErrOut,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})

	exitCode := 0
	exitMsg := "连接已断开"
	if err != nil {
		exitCode = 1
		exitMsg = "连接异常断开"
		l.Errorf("Stream Error: %v", err)
	} else {
		l.Infof("Stream 正常结束")
	}

	if !l.ws.IsClosed() {
		l.ws.SendMessage(wsutil.TypeExecExit, map[string]interface{}{
			"message": exitMsg,
			"code":    exitCode,
		})
	}

	return err
}

func (l *PodExecLogic) sendWsError(code, msg string) error {
	if !l.ws.IsClosed() {
		l.ws.SendErrorWithCode(code, msg)
	}
	return fmt.Errorf("%s: %s", code, msg)
}

func (l *PodExecLogic) connectionCheck(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.ws.CloseChan():
			return
		case <-ticker.C:
			if !l.ws.IsConnectionAlive() {
				return
			}
		}
	}
}

func (l *PodExecLogic) healthCheck(ctx context.Context, clusterUuid, namespace, podName, containerName string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.ws.CloseChan():
			return
		case <-ticker.C:
			client, err := l.svcCtx.K8sManager.GetCluster(ctx, clusterUuid)
			if err != nil {
				err := l.sendWsError("HEALTH_FAIL", "集群连接中断")
				if err != nil {
					return
				}
				return
			}
			pod, err := client.Pods().Get(namespace, podName)
			if err != nil {
				err := l.sendWsError("POD_GONE", "Pod 已消失")
				if err != nil {
					return
				}
				return
			}
			if pod.DeletionTimestamp != nil {
				err := l.sendWsError("POD_TERMINATING", "Pod 正在被删除")
				if err != nil {
					return
				}
				return
			}
		}
	}
}

func (l *PodExecLogic) handleResizeMessages(ctx context.Context, resizeChan <-chan *resizeMessage, sizeQueue *dynamicTerminalSizeQueue) {
	for {
		select {
		case <-ctx.Done():
			return
		case resize, ok := <-resizeChan:
			if !ok {
				return
			}
			sizeQueue.update(resize.Rows, resize.Cols)
		}
	}
}

type dynamicTerminalSizeQueue struct {
	mu       sync.RWMutex
	rows     uint16
	cols     uint16
	sizeChan chan *remotecommand.TerminalSize
	done     chan struct{}
}

func newDynamicTerminalSizeQueue(rows, cols uint16) *dynamicTerminalSizeQueue {
	q := &dynamicTerminalSizeQueue{
		rows:     rows,
		cols:     cols,
		sizeChan: make(chan *remotecommand.TerminalSize, 10),
		done:     make(chan struct{}),
	}
	q.sizeChan <- &remotecommand.TerminalSize{Width: cols, Height: rows}
	return q
}

func (q *dynamicTerminalSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case <-q.done:
		return nil
	case size := <-q.sizeChan:
		return size
	}
}

func (q *dynamicTerminalSizeQueue) update(rows, cols uint16) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rows = rows
	q.cols = cols
	select {
	case q.sizeChan <- &remotecommand.TerminalSize{Width: cols, Height: rows}:
	default:
	}
}

func (q *dynamicTerminalSizeQueue) close() {
	close(q.done)
}
