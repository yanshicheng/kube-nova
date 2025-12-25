package types

import (
	"context"
	"io"
	"time"
)

// ========== Pod 文件操作接口 ==========

// PodFileOperator Pod 文件操作器接口
// PodFileOperator Pod 文件操作器接口
type PodFileOperator interface {
	// ========== 文件浏览操作 ==========
	ListFiles(ctx context.Context, namespace, podName, container, path string, opts *FileListOptions) (*FileListResult, error)
	GetFileInfo(ctx context.Context, namespace, podName, container, path string) (*FileInfo, error)
	GetFileStats(ctx context.Context, namespace, podName, container, path string) (*FileStats, error)

	// ========== 文件下载操作 ==========
	DownloadFile(ctx context.Context, namespace, podName, container, filePath string, opts *DownloadOptions) (io.ReadCloser, error)
	DownloadFileChunk(ctx context.Context, namespace, podName, container, filePath string, chunkIndex int, chunkSize int64) ([]byte, error)

	// ========== 文件上传操作 ==========
	UploadFile(ctx context.Context, namespace, podName, container, destPath string, reader io.Reader, opts *UploadOptions) error

	// ========== 文件管理操作 ==========
	CreateDirectory(ctx context.Context, namespace, podName, container, dirPath string, opts *CreateDirOptions) error
	DeleteFiles(ctx context.Context, namespace, podName, container string, paths []string, opts *DeleteOptions) (*DeleteResult, error)
	MoveFile(ctx context.Context, namespace, podName, container, sourcePath, destPath string, overwrite bool) error
	CopyFile(ctx context.Context, namespace, podName, container, sourcePath, destPath string, opts *CopyOptions) error

	// ========== 文件内容操作 ==========
	ReadFile(ctx context.Context, namespace, podName, container, filePath string, opts *ReadOptions) (*FileContent, error)
	SaveFile(ctx context.Context, namespace, podName, container, filePath string, content []byte, opts *SaveOptions) error
	TailFile(ctx context.Context, namespace, podName, container, filePath string, opts *TailOptions) (chan string, chan error, error)

	// ========== 文件搜索操作 ==========
	SearchFiles(ctx context.Context, namespace, podName, container, searchPath string, opts *FileSearchOptions) (*FileSearchResponse, error)

	// ========== 辅助功能 ==========
	CheckFilePermission(ctx context.Context, namespace, podName, container, path string, permission FilePermission) (bool, error)
	CompressFiles(ctx context.Context, namespace, podName, container string, paths []string, destPath string, format CompressionFormat) error
	ExtractArchive(ctx context.Context, namespace, podName, container, archivePath, destPath string) error
}

// ========== 文件信息 ==========
// ========== 文件搜索相关 ==========

// FileSearchOptions 文件搜索选项
type FileSearchOptions struct {
	Pattern        string     `json:"pattern"`        // 文件名匹配模式（支持通配符）
	ContentSearch  string     `json:"contentSearch"`  // 内容搜索关键词
	FileTypes      []string   `json:"fileTypes"`      // 文件类型过滤
	MinSize        int64      `json:"minSize"`        // 最小文件大小
	MaxSize        int64      `json:"maxSize"`        // 最大文件大小
	ModifiedAfter  *time.Time `json:"modifiedAfter"`  // 修改时间晚于
	ModifiedBefore *time.Time `json:"modifiedBefore"` // 修改时间早于
	MaxDepth       int        `json:"maxDepth"`       // 最大搜索深度
	MaxResults     int        `json:"maxResults"`     // 最大结果数量
	CaseSensitive  bool       `json:"caseSensitive"`  // 是否大小写敏感
	FollowLinks    bool       `json:"followLinks"`    // 是否跟随符号链接
}

// ContentMatch 内容匹配信息
type ContentMatch struct {
	LineNumber int    `json:"lineNumber"` // 行号
	Line       string `json:"line"`       // 匹配的行内容
	Preview    string `json:"preview"`    // 上下文预览
}

// FileSearchResult 文件搜索结果
type FileSearchResult struct {
	Path    string         `json:"path"`    // 文件路径
	Name    string         `json:"name"`    // 文件名
	Size    int64          `json:"size"`    // 文件大小
	ModTime time.Time      `json:"modTime"` // 修改时间
	IsDir   bool           `json:"isDir"`   // 是否目录
	Matches []ContentMatch `json:"matches"` // 内容匹配（如果进行了内容搜索）
}

// FileSearchResponse 文件搜索响应
type FileSearchResponse struct {
	Results    []FileSearchResult `json:"results"`    // 搜索结果
	TotalFound int                `json:"totalFound"` // 找到的总数
	SearchTime int64              `json:"searchTime"` // 搜索耗时（毫秒）
	Truncated  bool               `json:"truncated"`  // 结果是否被截断
}

// FileInfo 文件信息
type FileInfo struct {
	Name         string         `json:"name"`         // 文件名
	Path         string         `json:"path"`         // 完整路径
	IsDir        bool           `json:"isDir"`        // 是否是目录
	Size         int64          `json:"size"`         // 文件大小（字节）
	Mode         string         `json:"mode"`         // 权限模式
	ModTime      time.Time      `json:"modTime"`      // 修改时间
	Owner        string         `json:"owner"`        // 所有者
	Group        string         `json:"group"`        // 所属组
	IsLink       bool           `json:"isLink"`       // 是否是符号链接
	LinkTarget   string         `json:"linkTarget"`   // 链接目标
	IsReadable   bool           `json:"isReadable"`   // 是否可读
	IsWritable   bool           `json:"isWritable"`   // 是否可写
	IsExecutable bool           `json:"isExecutable"` // 是否可执行
	MimeType     string         `json:"mimeType"`     // MIME类型
	Children     int            `json:"children"`     // 子项数量（目录）
	Permissions  FilePermission `json:"permissions"`  // 权限详情
}

// FilePermission 文件权限
type FilePermission struct {
	User  PermissionBits `json:"user"`  // 用户权限
	Group PermissionBits `json:"group"` // 组权限
	Other PermissionBits `json:"other"` // 其他权限
}

// PermissionBits 权限位
type PermissionBits struct {
	Read    bool `json:"read"`    // 读权限
	Write   bool `json:"write"`   // 写权限
	Execute bool `json:"execute"` // 执行权限
}

// FileSortBy 文件排序字段
type FileSortBy string

const (
	SortByName FileSortBy = "name" // 按名称排序
	SortBySize FileSortBy = "size" // 按大小排序
	SortByTime FileSortBy = "time" // 按时间排序
	SortByType FileSortBy = "type" // 按类型排序
)

// FileStats 文件统计信息
type FileStats struct {
	FileInfo   FileInfo  `json:"fileInfo"`   // 基本信息
	DiskUsage  int64     `json:"diskUsage"`  // 磁盘使用量
	FileCount  int       `json:"fileCount"`  // 文件数量
	DirCount   int       `json:"dirCount"`   // 目录数量
	TotalSize  int64     `json:"totalSize"`  // 总大小
	Checksum   string    `json:"checksum"`   // 校验和（MD5）
	LastAccess time.Time `json:"lastAccess"` // 最后访问时间
}

// BreadcrumbItem 面包屑项
type BreadcrumbItem struct {
	Name string `json:"name"` // 显示名称
	Path string `json:"path"` // 路径
}

// ========== 文件列表选项 ==========

// FileListOptions 文件列表选项
type FileListOptions struct {
	ShowHidden bool       `json:"showHidden"` // 显示隐藏文件
	SortBy     FileSortBy `json:"sortBy"`     // 排序字段
	SortDesc   bool       `json:"sortDesc"`   // 降序排序
	Search     string     `json:"search"`     // 搜索关键词
	FileTypes  []string   `json:"fileTypes"`  // 文件类型过滤
	MaxDepth   int        `json:"maxDepth"`   // 最大深度（用于递归）
	Recursive  bool       `json:"recursive"`  // 是否递归
	Limit      int        `json:"limit"`      // 返回数量限制
	Offset     int        `json:"offset"`     // 偏移量（分页）
}

// FileListResult 文件列表结果
type FileListResult struct {
	Files       []FileInfo       `json:"files"`       // 文件列表
	CurrentPath string           `json:"currentPath"` // 当前路径
	Breadcrumbs []BreadcrumbItem `json:"breadcrumbs"` // 面包屑导航
	TotalCount  int              `json:"totalCount"`  // 总数量
	TotalSize   int64            `json:"totalSize"`   // 总大小
	Container   string           `json:"container"`   // 容器名称
	HasMore     bool             `json:"hasMore"`     // 是否有更多
}

// ========== 下载选项 ==========

// DownloadOptions 下载选项
type DownloadOptions struct {
	Compress   bool  `json:"compress"`   // 是否压缩
	ChunkSize  int64 `json:"chunkSize"`  // 分块大小
	Resume     bool  `json:"resume"`     // 是否断点续传
	RangeStart int64 `json:"rangeStart"` // 范围开始
	RangeEnd   int64 `json:"rangeEnd"`   // 范围结束
}

// ========== 上传选项 ==========

// UploadOptions 上传选项
type UploadOptions struct {
	FileName    string `json:"fileName"`    // 文件名
	FileSize    int64  `json:"fileSize"`    // 文件大小
	FileMode    string `json:"fileMode"`    // 文件权限
	Overwrite   bool   `json:"overwrite"`   // 是否覆盖
	ChunkSize   int64  `json:"chunkSize"`   // 分块大小
	Checksum    string `json:"checksum"`    // 校验和
	CreateDirs  bool   `json:"createDirs"`  // 自动创建目录
	MaxFileSize int64  `json:"maxFileSize"` // 最大文件大小限制（新增字段）
}

// UploadStreamOptions 流式上传选项
type UploadStreamOptions struct {
	UploadOptions
	OnProgress func(progress UploadProgress) // 进度回调
}

// UploadSession 上传会话
type UploadSession struct {
	ID             string         `json:"id"`             // 会话ID
	FileName       string         `json:"fileName"`       // 文件名
	FilePath       string         `json:"filePath"`       // 文件路径
	FileSize       int64          `json:"fileSize"`       // 文件大小
	ChunkSize      int64          `json:"chunkSize"`      // 分块大小
	TotalChunks    int            `json:"totalChunks"`    // 总块数
	UploadedChunks int            `json:"uploadedChunks"` // 已上传块数
	BytesUploaded  int64          `json:"bytesUploaded"`  // 已上传字节数
	StartTime      time.Time      `json:"startTime"`      // 开始时间
	LastActivity   time.Time      `json:"lastActivity"`   // 最后活动时间
	Status         UploadStatus   `json:"status"`         // 状态
	Writer         io.WriteCloser // 写入器
	ErrorChan      chan error     // 错误通道
}

// UploadStatus 上传状态
type UploadStatus string

const (
	UploadStatusPending   UploadStatus = "pending"   // 等待中
	UploadStatusUploading UploadStatus = "uploading" // 上传中
	UploadStatusPaused    UploadStatus = "paused"    // 已暂停
	UploadStatusCompleted UploadStatus = "completed" // 已完成
	UploadStatusFailed    UploadStatus = "failed"    // 失败
	UploadStatusCanceled  UploadStatus = "canceled"  // 已取消
)

// UploadProgress 上传进度
type UploadProgress struct {
	SessionID      string        `json:"sessionId"`      // 会话ID
	BytesUploaded  int64         `json:"bytesUploaded"`  // 已上传字节
	TotalBytes     int64         `json:"totalBytes"`     // 总字节数
	ChunksUploaded int           `json:"chunksUploaded"` // 已上传块数
	TotalChunks    int           `json:"totalChunks"`    // 总块数
	Percentage     float64       `json:"percentage"`     // 百分比
	Speed          int64         `json:"speed"`          // 速度（字节/秒）
	TimeRemaining  time.Duration `json:"timeRemaining"`  // 剩余时间
}

// ========== 目录和删除选项 ==========

// CreateDirOptions 创建目录选项
type CreateDirOptions struct {
	Mode          string `json:"mode"`          // 权限模式（如：0755）
	CreateParents bool   `json:"createParents"` // 创建父目录
}

// DeleteOptions 删除选项
type DeleteOptions struct {
	Recursive bool `json:"recursive"` // 递归删除
	Force     bool `json:"force"`     // 强制删除
}

// DeleteResult 删除结果
type DeleteResult struct {
	DeletedCount int      `json:"deletedCount"` // 删除数量
	FailedPaths  []string `json:"failedPaths"`  // 失败路径
	Errors       []error  `json:"errors"`       // 错误列表
}

// CopyOptions 复制选项
type CopyOptions struct {
	Overwrite     bool `json:"overwrite"`     // 覆盖已存在文件
	Recursive     bool `json:"recursive"`     // 递归复制
	PreserveAttrs bool `json:"preserveAttrs"` // 保留属性
}

// ========== 文件内容操作 ==========

// ReadOptions 读取选项
type ReadOptions struct {
	Offset    int64  `json:"offset"`    // 偏移量
	Limit     int64  `json:"limit"`     // 限制大小
	Encoding  string `json:"encoding"`  // 编码
	Tail      bool   `json:"tail"`      // 从末尾读取
	TailLines int    `json:"tailLines"` // 末尾行数
}

// FileContent 文件内容
type FileContent struct {
	Content     string `json:"content"`     // 文本内容
	BinaryData  []byte `json:"binaryData"`  // 二进制数据
	FileSize    int64  `json:"fileSize"`    // 文件大小
	BytesRead   int64  `json:"bytesRead"`   // 读取字节数
	IsText      bool   `json:"isText"`      // 是否文本文件
	Encoding    string `json:"encoding"`    // 编码
	IsTruncated bool   `json:"isTruncated"` // 是否被截断
	LineCount   int    `json:"lineCount"`   // 行数（文本文件）
}

// SaveOptions 保存选项
type SaveOptions struct {
	CreateIfNotExists bool   `json:"createIfNotExists"` // 不存在时创建
	Backup            bool   `json:"backup"`            // 备份原文件
	Encoding          string `json:"encoding"`          // 编码
	FileMode          string `json:"fileMode"`          // 文件权限
}

// TailOptions 跟踪选项
type TailOptions struct {
	Lines     int  `json:"lines"`     // 初始行数
	Follow    bool `json:"follow"`    // 持续跟踪
	Retry     bool `json:"retry"`     // 文件不存在时重试
	MaxBuffer int  `json:"maxBuffer"` // 最大缓冲区
}

// ========== 压缩格式 ==========

// CompressionFormat 压缩格式
type CompressionFormat string

const (
	CompressionGzip CompressionFormat = "gzip" // Gzip格式
	CompressionZip  CompressionFormat = "zip"  // Zip格式
	CompressionTar  CompressionFormat = "tar"  // Tar格式
	CompressionTgz  CompressionFormat = "tgz"  // Tar.gz格式
)
