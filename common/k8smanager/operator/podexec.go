package operator

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ========== TerminalSizeQueue 适配器 ==========

// terminalSizeQueue 实现 TerminalSizeQueue 接口
type terminalSizeQueue struct {
	resizeChan <-chan remotecommand.TerminalSize
}

// Next 实现 TerminalSizeQueue 接口
func (t *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-t.resizeChan
	if !ok {
		return nil
	}
	return &size
}

// newTerminalSizeQueue 创建 TerminalSizeQueue
func newTerminalSizeQueue(resizeChan <-chan remotecommand.TerminalSize) remotecommand.TerminalSizeQueue {
	return &terminalSizeQueue{
		resizeChan: resizeChan,
	}
}

// ========== Pod 执行命令操作实现 ==========

// Exec 创建执行器用于执行命令
func (p *podOperator) Exec(namespace, name, container string, command []string, opts types.ExecOptions) (remotecommand.Executor, error) {
	if namespace == "" || name == "" {
		p.log.Error("创建执行器失败：命名空间和Pod名称不能为空")
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	if len(command) == 0 {
		p.log.Error("创建执行器失败：执行命令不能为空")
		return nil, fmt.Errorf("执行命令不能为空")
	}

	p.log.Infof("创建Pod执行器: namespace=%s, pod=%s, container=%s, command=%v",
		namespace, name, container, command)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	// 构建执行请求
	p.log.Debug("构建执行请求")
	req := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec")

	// 设置执行参数
	execParams := &corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     opts.Stdin,
		Stdout:    opts.Stdout,
		Stderr:    opts.Stderr,
		TTY:       opts.TTY,
	}

	p.log.Debugf("执行参数: stdin=%v, stdout=%v, stderr=%v, tty=%v",
		opts.Stdin, opts.Stdout, opts.Stderr, opts.TTY)

	req.VersionedParams(execParams, scheme.ParameterCodec)

	// 创建执行器
	p.log.Debug("创建SPDY执行器")
	executor, err := remotecommand.NewSPDYExecutor(p.config, "POST", req.URL())
	if err != nil {
		p.log.Errorf("创建执行器失败: %s/%s/%s, error=%v", namespace, name, container, err)
		return nil, fmt.Errorf("创建执行器失败: %v", err)
	}

	p.log.Infof("成功创建Pod执行器: %s/%s/%s", namespace, name, container)
	return executor, nil
}

// ExecWithStreams 执行命令并使用自定义流
func (p *podOperator) ExecWithStreams(namespace, name, container string, command []string, streams types.IOStreams) error {
	p.log.Infof("执行命令（带流）: namespace=%s, pod=%s, container=%s, command=%v",
		namespace, name, container, command)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return err
	}

	// 创建执行器
	executor, err := p.Exec(namespace, name, container, command, types.ExecOptions{
		Stdin:  streams.In != nil,
		Stdout: streams.Out != nil,
		Stderr: streams.ErrOut != nil,
		TTY:    false,
	})
	if err != nil {
		return err
	}

	// 执行命令
	p.log.Debug("开始流式执行命令")
	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  streams.In,
		Stdout: streams.Out,
		Stderr: streams.ErrOut,
		Tty:    false,
	})

	if err != nil {
		p.log.Errorf("执行命令失败: %s/%s/%s, error=%v", namespace, name, container, err)
		return fmt.Errorf("执行命令失败: %v", err)
	}

	p.log.Info("命令执行完成")
	return nil
}

// ExecCommand 执行命令并返回输出
func (p *podOperator) ExecCommand(namespace, name, container string, command []string) (stdout, stderr string, err error) {
	p.log.Infof("执行命令并获取输出: namespace=%s, pod=%s, container=%s, command=%v",
		namespace, name, container, command)

	// 确保有有效的容器名称
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return "", "", err
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	streams := types.IOStreams{
		In:     nil,
		Out:    &stdoutBuf,
		ErrOut: &stderrBuf,
	}

	err = p.ExecWithStreams(namespace, name, container, command, streams)

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		p.log.Errorf("执行命令失败: stdout长度=%d, stderr长度=%d, error=%v",
			len(stdout), len(stderr), err)
		if stderr != "" {
			p.log.Debugf("stderr内容: %s", stderr)
		}
	} else {
		p.log.Infof("命令执行成功，stdout长度=%d, stderr长度=%d", len(stdout), len(stderr))
	}

	return stdout, stderr, err
}

// ExecCommandWithTimeout 带超时的命令执行
func (p *podOperator) ExecCommandWithTimeout(
	ctx context.Context,
	namespace, name, container string,
	command []string,
	timeout time.Duration,
) (stdout, stderr string, err error) {

	p.log.Infof("执行带超时命令: namespace=%s, pod=%s, container=%s, timeout=%v, command=%v",
		namespace, name, container, timeout, command)

	if namespace == "" {
		return "", "", fmt.Errorf("命名空间不能为空")
	}
	if name == "" {
		return "", "", fmt.Errorf("Pod名称不能为空")
	}
	if len(command) == 0 {
		return "", "", fmt.Errorf("命令不能为空")
	}
	if timeout <= 0 {
		return "", "", fmt.Errorf("超时时间必须大于0")
	}

	// 确保有有效的容器名称
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return "", "", err
	}

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdoutBuf, stderrBuf bytes.Buffer

	// 创建执行器
	executor, err := p.Exec(namespace, name, container, command, types.ExecOptions{
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		TTY:    false,
	})
	if err != nil {
		return "", "", err
	}

	// 在 goroutine 中执行命令
	done := make(chan error, 1)
	go func() {
		// 注意：当context被取消时，executor.Stream应该检测到并返回
		done <- executor.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: &stdoutBuf,
			Stderr: &stderrBuf,
			Tty:    false,
		})
	}()

	// 等待执行完成或超时
	select {
	case <-timeoutCtx.Done():
		// Context已取消，goroutine应该会很快退出
		// 我们等待一小段时间让goroutine退出
		select {
		case <-done:
			// Goroutine已完成
		case <-time.After(100 * time.Millisecond):
			// Goroutine可能仍在运行，但我们已经返回
			p.log.Info("命令执行超时，goroutine可能仍在运行")
		}
		p.log.Errorf("命令执行超时: %s/%s/%s, timeout=%v", namespace, name, container, timeout)
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("命令执行超时: %v", timeout)
	case err := <-done:
		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()

		if err != nil {
			p.log.Errorf("命令执行失败: stdout长度=%d, stderr长度=%d, error=%v",
				len(stdout), len(stderr), err)
			return stdout, stderr, fmt.Errorf("命令执行失败: %v", err)
		}

		p.log.Infof("命令执行成功: stdout长度=%d, stderr长度=%d", len(stdout), len(stderr))
		return stdout, stderr, nil
	}
}

func (p *podOperator) ExecInteractiveShell(
	ctx context.Context,
	namespace, name, container string,
	streams types.IOStreams,
	resizeChan <-chan remotecommand.TerminalSize,
) error {

	if namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if name == "" {
		return fmt.Errorf("Pod名称不能为空")
	}

	p.log.Infof("创建交互式Shell: namespace=%s, pod=%s, container=%s", namespace, name, container)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return err
	}

	// 尝试使用不同的 shell
	shells := []string{"/bin/bash", "/bin/sh", "/bin/ash"}
	var lastErr error

	for _, shell := range shells {
		p.log.Debugf("尝试启动 shell: %s", shell)

		// 先检查 shell 是否存在
		testCmd := []string{"test", "-x", shell}
		_, _, err := p.ExecCommand(namespace, name, container, testCmd)
		if err != nil {
			p.log.Debugf("Shell 不存在或不可执行: %s", shell)
			continue
		}

		// 创建执行器
		command := []string{shell}
		executor, err := p.Exec(namespace, name, container, command, types.ExecOptions{
			Stdin:  true,
			Stdout: true,
			Stderr: true,
			TTY:    true, // 关键：分配 TTY
		})
		if err != nil {
			lastErr = err
			p.log.Errorf("创建执行器失败（%s）: %v", shell, err)
			continue
		}

		// 执行交互式会话
		p.log.Infof("成功启动交互式 Shell: %s", shell)

		// 创建 TerminalSizeQueue 适配器
		var sizeQueue remotecommand.TerminalSizeQueue
		if resizeChan != nil {
			sizeQueue = newTerminalSizeQueue(resizeChan)
		}

		streamOpts := remotecommand.StreamOptions{
			Stdin:             streams.In,
			Stdout:            streams.Out,
			Stderr:            streams.ErrOut,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		}

		err = executor.Stream(streamOpts)
		if err != nil {
			p.log.Errorf("交互式Shell会话错误: %v", err)
			return fmt.Errorf("交互式Shell会话错误: %v", err)
		}

		p.log.Info("交互式Shell会话结束")
		return nil
	}

	// 所有 shell 都失败
	if lastErr != nil {
		p.log.Errorf("无法启动任何Shell: %v", lastErr)
		return fmt.Errorf("无法启动任何Shell: %v", lastErr)
	}

	return fmt.Errorf("容器中没有找到可用的Shell")
}

// ValidateCommand 验证命令是否可以执行
func (p *podOperator) ValidateCommand(namespace, name, container string, command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("命令不能为空")
	}

	// 确保容器存在
	container, err := p.ensureContainer(namespace, name, container)
	if err != nil {
		return err
	}

	// 检查命令是否存在（仅检查第一个命令）
	cmdPath := command[0]
	testCmd := []string{"command", "-v", cmdPath}

	p.log.Debugf("验证命令是否存在: %s", cmdPath)
	_, _, err = p.ExecCommand(namespace, name, container, testCmd)
	if err != nil {
		p.log.Errorf("命令可能不存在或不可执行: %s", cmdPath)
		// 不返回错误，因为有些命令可能是内置的
	}

	return nil
}

// ExecCommandInAllContainers 在所有容器中执行命令
func (p *podOperator) ExecCommandInAllContainers(
	ctx context.Context,
	namespace, name string,
	command []string,
) (map[string]*types.ExecResponse, error) {

	if namespace == "" {
		return nil, fmt.Errorf("命名空间不能为空")
	}
	if name == "" {
		return nil, fmt.Errorf("Pod名称不能为空")
	}
	if len(command) == 0 {
		return nil, fmt.Errorf("命令不能为空")
	}

	p.log.Infof("在所有容器中执行命令: namespace=%s, pod=%s, command=%v",
		namespace, name, command)

	// 获取所有容器
	containers, err := p.GetAllContainers(namespace, name)
	if err != nil {
		return nil, err
	}

	results := make(map[string]*types.ExecResponse)

	// 在每个普通容器中执行
	for _, container := range containers.Containers {
		p.log.Debugf("在容器 %s 中执行命令", container.Name)
		startTime := time.Now()

		stdout, stderr, err := p.ExecCommand(namespace, name, container.Name, command)
		duration := time.Since(startTime).Milliseconds()

		response := &types.ExecResponse{
			Stdout:   []byte(stdout),
			Stderr:   []byte(stderr),
			Duration: duration,
		}

		if err != nil {
			response.Error = err.Error()
			response.ExitCode = 1
		} else {
			response.ExitCode = 0
		}

		results[container.Name] = response
	}

	p.log.Infof("命令执行完成，共 %d 个容器", len(results))
	return results, nil
}

// ExecBatch 批量执行命令
func (p *podOperator) ExecBatch(
	ctx context.Context,
	namespace, name, container string,
	commands [][]string,
) ([]*types.ExecResponse, error) {

	if namespace == "" {
		return nil, fmt.Errorf("命名空间不能为空")
	}
	if name == "" {
		return nil, fmt.Errorf("Pod名称不能为空")
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("命令列表不能为空")
	}

	p.log.Infof("批量执行命令: namespace=%s, pod=%s, container=%s, 命令数=%d",
		namespace, name, container, len(commands))

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	results := make([]*types.ExecResponse, 0, len(commands))

	for i, command := range commands {
		p.log.Debugf("执行第 %d/%d 个命令: %v", i+1, len(commands), command)
		startTime := time.Now()

		stdout, stderr, err := p.ExecCommand(namespace, name, container, command)
		duration := time.Since(startTime).Milliseconds()

		response := &types.ExecResponse{
			Stdout:   []byte(stdout),
			Stderr:   []byte(stderr),
			Duration: duration,
		}

		if err != nil {
			response.Error = err.Error()
			response.ExitCode = 1
		} else {
			response.ExitCode = 0
		}

		results = append(results, response)

		// 如果命令失败，可以选择是否继续
		if err != nil {
			p.log.Errorf("命令执行失败: %v", err)
		}
	}

	p.log.Infof("批量命令执行完成，共 %d 个命令", len(results))
	return results, nil
}

// ExecScript 执行脚本
func (p *podOperator) ExecScript(
	ctx context.Context,
	namespace, name, container string,
	script string,
	interpreter string,
) (*types.ExecResponse, error) {

	if namespace == "" {
		return nil, fmt.Errorf("命名空间不能为空")
	}
	if name == "" {
		return nil, fmt.Errorf("Pod名称不能为空")
	}
	if script == "" {
		return nil, fmt.Errorf("脚本内容不能为空")
	}

	p.log.Infof("执行脚本: namespace=%s, pod=%s, container=%s, 解释器=%s",
		namespace, name, container, interpreter)

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, name, container)
	if err != nil {
		return nil, err
	}

	// 默认使用 sh
	if interpreter == "" {
		interpreter = "/bin/sh"
	}

	// 构建执行命令
	command := []string{interpreter, "-c", script}

	startTime := time.Now()
	stdout, stderr, err := p.ExecCommand(namespace, name, container, command)
	duration := time.Since(startTime).Milliseconds()

	response := &types.ExecResponse{
		Stdout:   []byte(stdout),
		Stderr:   []byte(stderr),
		Duration: duration,
	}

	if err != nil {
		response.Error = err.Error()
		response.ExitCode = 1
		p.log.Errorf("脚本执行失败: %v", err)
	} else {
		response.ExitCode = 0
		p.log.Infof("脚本执行成功，耗时: %dms", duration)
	}

	return response, nil
}

// CheckCommandExists 检查命令是否存在
func (p *podOperator) CheckCommandExists(namespace, name, container, cmdName string) (bool, error) {
	p.log.Debugf("检查命令是否存在: %s/%s/%s, cmd=%s", namespace, name, container, cmdName)

	testCmd := []string{"command", "-v", cmdName}
	_, _, err := p.ExecCommand(namespace, name, container, testCmd)

	exists := err == nil
	p.log.Debugf("命令 %s 存在: %v", cmdName, exists)

	return exists, nil
}

// GetShellType 获取容器中可用的 Shell 类型
func (p *podOperator) GetShellType(namespace, name, container string) (string, error) {
	p.log.Debugf("获取Shell类型: %s/%s/%s", namespace, name, container)

	shells := []string{"/bin/bash", "/bin/sh", "/bin/ash", "/bin/zsh"}

	for _, shell := range shells {
		testCmd := []string{"test", "-x", shell}
		_, _, err := p.ExecCommand(namespace, name, container, testCmd)
		if err == nil {
			p.log.Infof("找到可用Shell: %s", shell)
			return shell, nil
		}
	}

	p.log.Error("未找到可用的Shell")
	return "", fmt.Errorf("未找到可用的Shell")
}

// ExecWithRetry 带重试的命令执行
func (p *podOperator) ExecWithRetry(
	ctx context.Context,
	namespace, name, container string,
	command []string,
	maxRetries int,
	retryDelay time.Duration,
) (stdout, stderr string, err error) {

	p.log.Infof("带重试执行命令: namespace=%s, pod=%s, container=%s, 最大重试=%d",
		namespace, name, container, maxRetries)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			p.log.Infof("重试第 %d 次，等待 %v", attempt, retryDelay)
			time.Sleep(retryDelay)
		}

		stdout, stderr, err = p.ExecCommand(namespace, name, container, command)
		if err == nil {
			p.log.Infof("命令执行成功，尝试次数: %d", attempt+1)
			return stdout, stderr, nil
		}

		p.log.Errorf("命令执行失败（尝试 %d/%d）: %v", attempt+1, maxRetries+1, err)
	}

	p.log.Errorf("命令执行失败，已达最大重试次数: %d", maxRetries)
	return stdout, stderr, fmt.Errorf("命令执行失败，已重试 %d 次: %v", maxRetries, err)
}
