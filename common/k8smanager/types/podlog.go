package types

import (
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ========== Pod 日志操作接口 ==========

// PodLogOperator Pod 日志操作器接口
type PodLogOperator interface {
	// GetLogs 获取 Pod 日志流
	GetLogs(namespace, name, container string, opts *corev1.PodLogOptions) (io.ReadCloser, error)

	// GetLogsWithFollow 获取 Pod 日志并持续跟踪
	GetLogsWithFollow(namespace, name, container string, opts *LogsOptions) (io.ReadCloser, error)

	// DownloadLogs 下载 Pod 日志
	DownloadLogs(namespace, name string, opts *LogsDownloadOptions) ([]byte, error)
}

// ========== 日志选项 ==========

// LogsOptions 日志选项
type LogsOptions struct {
	Container                    string       // 容器名称，为空则自动选择第一个容器
	Follow                       bool         // 是否持续跟踪
	Previous                     bool         // 是否获取前一个容器的日志
	SinceSeconds                 *int64       // 获取多少秒内的日志
	SinceTime                    *metav1.Time // 从某个时间点开始的日志
	Timestamps                   bool         // 是否包含时间戳
	TailLines                    *int64       // 🔥 尾部行数，nil=全部，0=全部，>0=指定行数
	LimitBytes                   *int64       // 限制字节数
	InsecureSkipTLSVerifyBackend bool         // 跳过TLS验证
}

// LogsDownloadOptions 日志下载选项
type LogsDownloadOptions struct {
	// 容器选项
	Container     string // 指定容器名称
	AllContainers bool   // 是否下载所有容器的日志

	// 时间范围
	TimeRange struct {
		Start *time.Time
		End   *time.Time
	}

	// 获取选项
	TailLines  int64 // 只获取最后 N 行（0=全部）
	Previous   bool  // 获取之前的日志（容器重启前）
	Timestamps bool  // 是否包含时间戳

	// 限制选项
	MaxSize         int64 // 最大文件大小（字节）
	ContinueOnError bool  // 遇到错误是否继续
}

// LogStreamOptions 日志流选项（用于实时流式获取）
type LogStreamOptions struct {
	Container  string // 容器名称
	Follow     bool   // 是否跟踪
	Previous   bool   // 是否获取之前的日志
	Timestamps bool   // 是否包含时间戳
	TailLines  *int64 // 🔥 尾部行数，nil=全部，0=全部，>0=指定行数
}
