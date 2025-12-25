// common/uploadcore/types.go
package uploadcore

import (
	"context"
	"time"
)

// UploadSession 上传会话
type UploadSession struct {
	SessionID      string
	UploadType     string // pod_file, image, oss 等
	FileName       string
	FileSize       int64
	ChunkSize      int64
	TotalChunks    int
	UploadedChunks map[int]bool
	TempFilePath   string
	Metadata       map[string]interface{} // 业务元数据
	Status         string
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastActivity   time.Time
}

// UploadCompleteCallback 完成上传的回调函数（业务实现）
type UploadCompleteCallback func(ctx context.Context, session *UploadSession, tempFilePath string) (targetPath string, metadata map[string]string, err error)

// CompleteResult 完成上传的结果
type CompleteResult struct {
	TargetPath string            // 目标路径/URL
	FileSize   int64             // 文件大小
	Checksum   string            // 校验和
	Metadata   map[string]string // 结果元数据
	UploadTime int64             // 上传耗时(ms)
}
