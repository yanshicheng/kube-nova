package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadImageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadImageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadImageLogic {
	return &UploadImageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UploadImageLogic) UploadImage(r *http.Request) (resp *types.UploadImageResponse, err error) {

	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	} // 解析 multipart 表单，设置最大内存大小 32MB
	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		l.Errorf("解析multipart表单失败: %v", err)
		return nil, fmt.Errorf("解析表单失败: %w", err)
	}

	// 打印所有表单字段用于调试
	l.Infof("表单字段: %v", r.MultipartForm.File)
	for key, files := range r.MultipartForm.File {
		l.Infof("字段名: %s, 文件数量: %d", key, len(files))
		for i, file := range files {
			l.Infof("文件 %d: %s, 大小: %d", i, file.Filename, file.Size)
		}
	}

	// 尝试从 'image' 字段获取文件
	image, handler, err := r.FormFile("image")
	if err != nil {
		l.Errorf("获取文件失败: %v", err)

		// 如果 'image' 字段不存在，尝试其他可能的字段名
		if errors.Is(err, http.ErrMissingFile) {
			for fieldName := range r.MultipartForm.File {
				l.Infof("尝试使用字段: %s", fieldName)
				image, handler, err = r.FormFile(fieldName)
				if err == nil {
					l.Infof("成功从字段 %s 获取文件", fieldName)
					break
				}
			}
		}

		if err != nil {
			return nil, fmt.Errorf("未找到文件: %w", err)
		}
	}

	defer func(image multipart.File) {
		if closeErr := image.Close(); closeErr != nil {
			l.Errorf("关闭文件失败: %v", closeErr)
		}
	}(image)

	l.Infof("上传的文件名: %s, 大小: %d", handler.Filename, handler.Size)

	// 验证文件类型
	if !isValidImageType(handler.Header.Get("Content-Type")) {
		return nil, fmt.Errorf("不支持的文件类型: %s", handler.Header.Get("Content-Type"))
	}

	// 验证文件大小 (5MB)
	if handler.Size > 5*1024*1024 {
		return nil, fmt.Errorf("文件大小超过限制: %d bytes", handler.Size)
	}

	// 读取文件内容
	imageBytes, err := readFile(image, handler.Size)
	if err != nil {
		l.Errorf("读取文件失败: %v", err)
		return nil, err
	}

	// 并发处理上传请求
	res, err := l.uploadImageAsync(imageBytes, handler.Filename, string("users"))
	if err != nil {
		l.Errorf("上传图片失败: %v", err)
		return nil, err
	}

	l.Infof("图片上传成功: %v", res)
	return &types.UploadImageResponse{
		ImageUri: res.ImageUri,
		ImageUrl: res.ImageUrl,
	}, nil
}

// 验证图片类型
func isValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
	}

	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}

// 读取文件内容的函数，避免重复代码
func readFile(image io.Reader, size int64) ([]byte, error) {
	imageBytes := make([]byte, size)
	n, err := io.ReadFull(image, imageBytes)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 如果实际读取的字节数小于预期，调整切片大小
	if int64(n) < size {
		imageBytes = imageBytes[:n]
	}

	return imageBytes, nil
}

// 异步上传图片的函数
func (l *UploadImageLogic) uploadImageAsync(imageData []byte, fileName string, project string) (*pb.UploadImageResponse, error) {
	// 使用 goroutine 并发上传，处理完成后返回结果
	var wg sync.WaitGroup
	var res *pb.UploadImageResponse
	var err error

	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err = l.svcCtx.StoreRpc.UploadImage(l.ctx, &pb.UploadImageRequest{
			ImageData: imageData,
			FileName:  fileName,
			Project:   project, // 可以根据实际项目传递动态参数
		})
	}()
	wg.Wait()

	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}

	return res, nil
}
