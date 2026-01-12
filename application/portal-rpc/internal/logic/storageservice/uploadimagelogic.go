package storageservicelogic

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/code"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type UploadImageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadImageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadImageLogic {
	return &UploadImageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UploadImage 上传图片
func (l *UploadImageLogic) UploadImage(in *pb.UploadImageRequest) (*pb.UploadImageResponse, error) {
	// MinIO 初始化检查
	err := l.checkMinIO()
	if err != nil {
		l.Errorf("MinIO 检查失败: %v", err)
		return nil, code.MinioCheckErr
	}
	// 处理上传的图片数据
	l.Infof("开始上传图片: %s", in.FileName)
	bucketName := in.Project         // 这里使用请求中的项目名称作为桶名称
	ext := filepath.Ext(in.FileName) // 获取文件扩展名，包含点号
	dirName := time.Now().Format("20060102")
	fileName := time.Now().Format("20060102150405")
	absFileName := fmt.Sprintf("%s/%s", dirName, fileName)
	objectKey := fmt.Sprintf("/%s/%s%s", in.Project, absFileName, ext) // 存储路径为 project/时间戳.扩展名
	imageData := in.ImageData
	dataSize := int64(len(imageData)) // 获取文件大小
	// 将图片数据上传到 MinIO
	// 设置 Content-Type
	contentType := getContentType(ext)
	err = l.svcCtx.Storage.Upload(l.svcCtx.Config.StorageConf.BucketName, objectKey, bytes.NewReader(imageData), dataSize, contentType)
	if err != nil {
		l.Errorf("图片上传失败, 项目: %s, 文件名: %s, 错误: %v", bucketName, objectKey, err)
		return nil, code.MinioUploadErr
	}
	l.Infof("图片上传成功, 项目: %s, 文件名: %s", bucketName, objectKey)

	return &pb.UploadImageResponse{
		ImageUri: objectKey,
		ImageUrl: fmt.Sprintf("%s/%s/%s", l.svcCtx.Config.StorageConf.EndpointProxy, l.svcCtx.Config.StorageConf.BucketName, objectKey),
	}, nil
}

// 封装 MinIO 初始化检查的函数
func (l *UploadImageLogic) checkMinIO() error {
	xs := l.svcCtx.Storage.Ping()
	if xs != nil {
		return fmt.Errorf("MinIO 初始化异常: %v", xs)
	}
	l.Info("MinIO 初始化正常")
	return nil
}

// 根据文件扩展名获取对应的 Content-Type
func getContentType(ext string) string {
	// 这里你可以根据不同的扩展名返回对应的 MIME 类型
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream" // 默认类型
	}
}
