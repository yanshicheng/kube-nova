package pod

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	w      http.ResponseWriter
	r      *http.Request
}

func NewPodFileDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext, w http.ResponseWriter, r *http.Request) *PodFileDownloadLogic {
	return &PodFileDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		w:      w,
		r:      r,
	}
}

func (l *PodFileDownloadLogic) PodFileDownload(req *types.PodFileDownloadReq) error {
	// 1. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return fmt.Errorf("获取默认容器失败")
		}
	}

	fileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取文件信息失败: %v", err)
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	rangeHeader := l.r.Header.Get("Range")
	var rangeStart, rangeEnd int64
	supportsRange := false

	if rangeHeader != "" && strings.HasPrefix(rangeHeader, "bytes=") {
		// 解析 Range 头: "bytes=0-1023"
		rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.Split(rangeStr, "-")
		if len(parts) == 2 {
			rangeStart, _ = strconv.ParseInt(parts[0], 10, 64)
			if parts[1] != "" {
				rangeEnd, _ = strconv.ParseInt(parts[1], 10, 64)
			} else {
				rangeEnd = fileInfo.Size - 1
			}
			supportsRange = true
			l.Infof("断点续传请求: %d-%d", rangeStart, rangeEnd)
		}
	}

	// 7. 构建下载选项
	downloadOpts := &k8stypes.DownloadOptions{
		Compress:   req.Compress,
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
	}

	// 8. 下载文件
	reader, err := podClient.DownloadFile(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, downloadOpts)
	if err != nil {
		l.Errorf("下载文件失败: %v", err)
		return fmt.Errorf("下载文件失败: %v", err)
	}
	defer reader.Close()

	filename := filepath.Base(req.Path)

	// 处理文件名编码（支持中文）
	encodedFilename := url.QueryEscape(filename)

	if req.Compress {
		filename = filename + ".gz"
		encodedFilename = url.QueryEscape(filename)
		l.w.Header().Set("Content-Type", "application/gzip")
	} else {
		// 根据文件扩展名设置 MIME 类型
		contentType := getContentType(filename)
		l.w.Header().Set("Content-Type", contentType)
	}

	// RFC 5987 编码（支持中文文件名）
	l.w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s",
			filename, encodedFilename))

	if supportsRange {
		contentLength := rangeEnd - rangeStart + 1
		l.w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
		l.w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, fileInfo.Size))
		l.w.Header().Set("Accept-Ranges", "bytes")
		l.w.WriteHeader(http.StatusPartialContent) // 206
	} else {
		if !req.Compress {
			l.w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
		}
		l.w.Header().Set("Accept-Ranges", "bytes")
		l.w.WriteHeader(http.StatusOK) // 200
	}

	written, err := io.Copy(l.w, reader)
	if err != nil {
		l.Errorf("传输文件失败: %v", err)
		return fmt.Errorf("传输文件失败: %v", err)
	}

	l.Infof("成功下载文件: %s, 传输: %d bytes, 压缩: %v, 断点续传: %v",
		req.Path, written, req.Compress, supportsRange)

	return nil
}

func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}
