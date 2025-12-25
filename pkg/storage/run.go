package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/yanshicheng/kube-nova/pkg/storage/minio"
	"github.com/zeromicro/go-zero/core/logx"
)

// UploaderOptions 定义了配置上传器所需的参数
type UploaderOptions struct {
	Provider      string   // 存储提供商类型，例如 aliyun、minio
	Endpoints     []string // 端点地址
	EndpointProxy string
	AccessKey     string // 访问密钥
	AccessSecret  string // 访问密钥的秘钥
	CAFile        string // TLS CA证书文件路径
	CAKey         string // TLS CA密钥文件路径
	UseTLS        bool   // 是否启用 TLS
	BucketName    string
}

// 定义 Uploader 接口
type Uploader interface {
	Upload(bucketName, objectKey string, data io.Reader, size int64, contentType string) error
	Ping() error
}

// LogicService 实现了 service 接口，包含日志功能和上传器客户端
type LogicService struct {
	L        logx.Logger
	Uploader Uploader
}

// NewUploader 创建一个新的上传器客户端
func NewUploader(opts UploaderOptions) (Uploader, error) {
	switch opts.Provider {
	case "aliyun":
		// 阿里云 OSS 存储服务初始化（此处省略）
		return nil, fmt.Errorf("aliyun uploader not implemented")
	case "tencent":
		// 腾讯云存储服务初始化（此处省略）
		return nil, fmt.Errorf("tencent uploader not implemented")
	case "minio":
		// MinIO 存储服务初始化
		uploader, err := minio.NewMinioStore(opts.Endpoints[0], opts.AccessKey, opts.AccessSecret, opts.CAFile, opts.CAKey, opts.UseTLS)
		if err != nil {
			return nil, fmt.Errorf("创建 MinIO 客户端失败: %v", err)
		}
		return uploader, nil
	default:
		return nil, fmt.Errorf("不支持的存储提供商: %s", opts.Provider)
	}
}

// Run 方法使用已初始化的上传器客户端上传文件
func (l *LogicService) Run(bucketName, objectName string, data io.Reader, dataSize int64, contextType string) error {
	if l.Uploader == nil {
		return fmt.Errorf("上传器客户端未初始化")
	}

	// 上传文件
	err := l.Uploader.Upload(bucketName, objectName, data, dataSize, contextType)
	if err != nil {
		l.L.Errorf("上传文件失败, err: %v", err)
		return err
	}

	l.L.Infof("文件上传成功, bucket: %s, object: %s", bucketName, objectName)
	return nil
}

// NewLogicService 创建一个 LogicService 实例，并初始化日志对象和上传器客户端
func NewLogicService(uploader Uploader) *LogicService {
	return &LogicService{
		L:        logx.WithContext(context.Background()), // 初始化日志组件
		Uploader: uploader,
	}
}
