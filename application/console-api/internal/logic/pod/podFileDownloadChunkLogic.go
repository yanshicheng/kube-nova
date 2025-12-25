package pod

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileDownloadChunkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 分块下载文件
func NewPodFileDownloadChunkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileDownloadChunkLogic {
	return &PodFileDownloadChunkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileDownloadChunkLogic) PodFileDownloadChunk(req *types.PodFileDownloadChunkReq) (resp *types.PodFileDownloadChunkResp, err error) {
	// 1. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
	}

	// 5. 获取文件信息以计算总块数
	fileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取文件信息失败: %v", err)
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 6. 计算块信息
	chunkSize := req.ChunkSize
	if chunkSize == 0 {
		chunkSize = 1048576 // 默认 1MB
	}
	totalChunks := int((fileInfo.Size + chunkSize - 1) / chunkSize)

	// 7. 验证块索引
	if req.ChunkIndex < 0 || req.ChunkIndex >= totalChunks {
		return nil, fmt.Errorf("块索引超出范围: %d, 总块数: %d", req.ChunkIndex, totalChunks)
	}

	// 8. 下载指定块
	chunkData, err := podClient.DownloadFileChunk(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, req.ChunkIndex, chunkSize)
	if err != nil {
		l.Errorf("下载文件块失败: %v", err)
		return nil, fmt.Errorf("下载文件块失败: %v", err)
	}

	// 9. Base64 编码
	encodedData := base64.StdEncoding.EncodeToString(chunkData)

	// 10. 计算校验和（MD5）
	checksum := fmt.Sprintf("%x", md5.Sum(chunkData))

	// 11. 构建响应
	resp = &types.PodFileDownloadChunkResp{
		Data:        encodedData,
		ChunkIndex:  req.ChunkIndex,
		ChunkSize:   int64(len(chunkData)),
		TotalChunks: totalChunks,
		FileSize:    fileInfo.Size,
		IsLast:      req.ChunkIndex == totalChunks-1,
		Checksum:    checksum,
	}

	l.Infof("成功下载文件块: %s, 块索引: %d/%d, 大小: %d", req.Path, req.ChunkIndex, totalChunks, len(chunkData))

	return resp, nil
}
