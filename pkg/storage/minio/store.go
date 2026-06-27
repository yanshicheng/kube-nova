package minio

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStore  minio存储
type MinioStore struct {
	client *minio.Client
}

// 确保 MinioStore 实现了 storage.Uploader 接口
var (
// _ storage.Uploader = &MinioStore{}
)

// NewMinioStore 创建一个 MinioStore 实例，支持传入 TLS 相关的配置
func NewMinioStore(endpoint, accessKey, accessSecret, CAFile, clientCertFile, clientKeyFile string, useTLS bool) (*MinioStore, error) {
	var tlsConfig *tls.Config
	if CAFile != "" || clientCertFile != "" || clientKeyFile != "" {
		tlsConfig = &tls.Config{}
	}
	if CAFile != "" {
		// 加载 CA 证书
		caCert, err := os.ReadFile(CAFile)
		if err != nil {
			return nil, fmt.Errorf("加载 CA 证书失败: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("CA 证书格式不正确")
		}

		tlsConfig.RootCAs = caCertPool
	}
	if clientCertFile != "" || clientKeyFile != "" {
		clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书失败: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	// 创建 MinIO 客户端
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, accessSecret, ""),
		Secure: useTLS, // 根据 useTLS 参数决定是否启用安全连接
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端失败: %v", err)
	}
	return &MinioStore{
		client: minioClient,
	}, nil
}

// Upload 方法实现文件上传
func (m *MinioStore) Upload(bucketName, objectKey string, data io.Reader, size int64, contentType string) error {
	ctx := context.Background()

	// 检查桶是否存在，如果不存在则创建
	exists, err := m.client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("检查桶是否存在失败: %v", err)
	}
	if !exists {
		if err := m.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("创建桶失败: %v", err)
		}
	}

	// 上传文件到 MinIO
	uploadInfo, err := m.client.PutObject(ctx, bucketName, objectKey, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("上传文件失败: %v", err)
	}

	fmt.Printf("Uploaded %s/%s, size: %v bytes, ETag: %s\n", bucketName, objectKey, uploadInfo.Size, uploadInfo.ETag)
	return nil
}

// Download 方法实现文件下载
func (m *MinioStore) Download(ctx context.Context, bucketName, objectKey string) (io.ReadCloser, int64, string, error) {
	info, err := m.client.StatObject(ctx, bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, 0, "", fmt.Errorf("获取对象信息失败: %v", err)
	}
	object, err := m.client.GetObject(ctx, bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", fmt.Errorf("下载对象失败: %v", err)
	}
	return object, info.Size, info.ContentType, nil
}

// Ping 方法检查 MinIO 服务器的可用性
func (m *MinioStore) Ping() error {
	ctx := context.Background()
	// 尝试获取一个桶的信息，以确认连接正常
	buckets, err := m.client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("无法连接到 MinIO 服务器: %v", err)
	}

	// 如果能够获取到桶的信息，则表示连接正常
	fmt.Printf("S3 兼容对象存储可用, 当前桶数量: %d\n", len(buckets))
	return nil
}
