package types

import (
	"io"

	"k8s.io/client-go/tools/remotecommand"
)

// ========== Pod 执行命令操作接口 ==========

// PodExecOperator Pod 执行命令操作器接口
type PodExecOperator interface {
	// Exec 创建执行器用于执行命令
	Exec(namespace, name, container string, command []string, opts ExecOptions) (remotecommand.Executor, error)

	// ExecWithStreams 执行命令并使用自定义流
	ExecWithStreams(namespace, name, container string, command []string, streams IOStreams) error

	// ExecCommand 执行命令并返回输出
	ExecCommand(namespace, name, container string, command []string) (stdout, stderr string, err error)
}

// ========== 执行选项 ==========

// ExecOptions 执行选项
type ExecOptions struct {
	Stdin              bool              // 是否启用标准输入
	Stdout             bool              // 是否启用标准输出
	Stderr             bool              // 是否启用标准错误
	TTY                bool              // 是否分配 TTY
	Container          string            // 容器名称
	Command            []string          // 执行命令
	Env                map[string]string // 环境变量
	WorkingDir         string            // 工作目录
	TimeoutSeconds     int               // 超时时间（秒）
	PreserveWhitespace bool              // 保留空白字符
}

// IOStreams 输入输出流
type IOStreams struct {
	In     io.Reader // 标准输入
	Out    io.Writer // 标准输出
	ErrOut io.Writer // 标准错误
}

// ========== 执行请求响应（非交互式） ==========

// ExecRequest 执行命令请求
type ExecRequest struct {
	Namespace string   `json:"namespace"` // 命名空间
	PodName   string   `json:"podName"`   // Pod名称
	Container string   `json:"container"` // 容器名称
	Command   []string `json:"command"`   // 命令
	Stdin     []byte   `json:"stdin"`     // 标准输入（可选）
	TTY       bool     `json:"tty"`       // 是否分配TTY
	Timeout   int      `json:"timeout"`   // 超时时间（秒）
}

// ExecResponse 执行命令响应
type ExecResponse struct {
	Stdout   []byte `json:"stdout"`   // 标准输出
	Stderr   []byte `json:"stderr"`   // 标准错误
	ExitCode int32  `json:"exitCode"` // 退出码
	Error    string `json:"error"`    // 错误信息
	Duration int64  `json:"duration"` // 执行时长（毫秒）
}
