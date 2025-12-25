package minio

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
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
func NewMinioStore(endpoint, accessKey, accessSecret, CAFile, CAKey string, useTLS bool) (*MinioStore, error) {
	var tlsConfig *tls.Config
	if CAFile != "" {
		// 加载CA证书
		caCert, err := os.ReadFile(CAFile)
		if err != nil {
			log.Fatalln(err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// 加载客户端证书
		clientCert, err := tls.LoadX509KeyPair(CAFile, CAKey)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书失败: %v", err)
		}

		// 设置TLS配置
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caCertPool,
		}
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

	log.Printf("Uploaded %s/%s, size: %v bytes, ETag: %s\n", bucketName, objectKey, uploadInfo.Size, uploadInfo.ETag)
	return nil
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
	log.Printf("MinIO 服务器可用, 当前桶数量: %d\n", len(buckets))
	return nil
}
