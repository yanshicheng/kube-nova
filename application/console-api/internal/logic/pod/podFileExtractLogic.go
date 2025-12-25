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

type PodFileExtractLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 解压文件
func NewPodFileExtractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileExtractLogic {
	return &PodFileExtractLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileExtractLogic) PodFileExtract(req *types.PodFileExtractReq) (resp *types.PodFileExtractResp, err error) {
	startTime := time.Now()

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

	// 5. 解压文件
	err = podClient.ExtractArchive(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.ArchivePath, req.DestPath)
	if err != nil {
		l.Errorf("解压文件失败: %v", err)
		return nil, fmt.Errorf("解压文件失败: %v", err)
	}

	// 6. 获取解压后的目录统计信息
	stats, err := podClient.GetFileStats(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.DestPath)
	if err != nil {
		l.Errorf("获取解压目录统计信息失败: %v", err)
		stats = &k8stypes.FileStats{
			FileCount: 0,
			TotalSize: 0,
		}
	}

	// 7. 计算耗时
	duration := time.Since(startTime).Milliseconds()

	// 8. 构建响应
	resp = &types.PodFileExtractResp{
		DestPath:       req.DestPath,
		ExtractedFiles: stats.FileCount,
		TotalSize:      stats.TotalSize,
		Duration:       duration,
	}

	l.Infof("解压完成: %s -> %s, 文件数: %d, 大小: %d, 耗时: %dms",
		req.ArchivePath, req.DestPath, stats.FileCount, stats.TotalSize, duration)

	return resp, nil
}
