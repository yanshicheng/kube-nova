package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileCompressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 压缩文件或目录
func NewPodFileCompressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileCompressLogic {
	return &PodFileCompressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileCompressLogic) PodFileCompress(req *types.PodFileCompressReq) (resp *types.PodFileCompressResp, err error) {
	startTime := time.Now()

	// 1. 验证参数
	if len(req.Paths) == 0 {
		return nil, fmt.Errorf("压缩路径列表不能为空")
	}

	// 2. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 3. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 4. 初始化 Pod 操作器
	podClient := client.Pods()

	// 5. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
	}

	// 6. 解析压缩格式
	var format k8stypes.CompressionFormat
	switch req.Format {
	case "gzip":
		format = k8stypes.CompressionGzip
	case "zip":
		format = k8stypes.CompressionZip
	case "tar":
		format = k8stypes.CompressionTar
	case "tgz", "":
		format = k8stypes.CompressionTgz
	default:
		return nil, fmt.Errorf("不支持的压缩格式: %s", req.Format)
	}

	// 7. 计算原始大小
	var originalSize int64
	fileCount := 0
	for _, path := range req.Paths {
		stats, err := podClient.GetFileStats(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, path)
		if err != nil {
			l.Errorf("获取文件统计信息失败: %s, %v", path, err)
			continue
		}
		originalSize += stats.TotalSize
		fileCount += stats.FileCount
	}

	// 8. 压缩文件
	err = podClient.CompressFiles(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Paths, req.DestPath, format)
	if err != nil {
		l.Errorf("压缩文件失败: %v", err)
		return nil, fmt.Errorf("压缩文件失败: %v", err)
	}

	// 9. 获取压缩包信息
	archiveInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.DestPath)
	if err != nil {
		l.Errorf("获取压缩包信息失败: %v", err)
		archiveInfo = &k8stypes.FileInfo{Size: 0}
	}

	// 10. 计算压缩率
	compressionRatio := 0.0
	if originalSize > 0 {
		compressionRatio = float64(archiveInfo.Size) / float64(originalSize)
	}

	// 11. 计算耗时
	duration := time.Since(startTime).Milliseconds()

	// 12. 构建响应
	resp = &types.PodFileCompressResp{
		ArchivePath:      req.DestPath,
		FileCount:        fileCount,
		OriginalSize:     originalSize,
		CompressedSize:   archiveInfo.Size,
		CompressionRatio: compressionRatio,
		Duration:         duration,
	}

	l.Infof("压缩完成: %s, 文件数: %d, 原始大小: %d, 压缩后: %d, 压缩率: %.2f%%, 耗时: %dms",
		req.DestPath, fileCount, originalSize, archiveInfo.Size, compressionRatio*100, duration)

	return resp, nil
}
