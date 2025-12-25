package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取文件统计信息
func NewPodFileStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileStatsLogic {
	return &PodFileStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileStatsLogic) PodFileStats(req *types.PodFileStatsReq) (resp *types.PodFileStatsResp, err error) {
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

	// 5. 获取文件统计信息
	stats, err := podClient.GetFileStats(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取文件统计信息失败: %v", err)
		return nil, fmt.Errorf("获取文件统计信息失败: %v", err)
	}

	// 6. 转换为响应格式
	resp = &types.PodFileStatsResp{
		FileInfo: types.FileInfo{
			Name:         stats.FileInfo.Name,
			Path:         stats.FileInfo.Path,
			IsDir:        stats.FileInfo.IsDir,
			Size:         stats.FileInfo.Size,
			Mode:         stats.FileInfo.Mode,
			ModTime:      stats.FileInfo.ModTime.Format("2006-01-02T15:04:05Z07:00"),
			Owner:        stats.FileInfo.Owner,
			Group:        stats.FileInfo.Group,
			IsLink:       stats.FileInfo.IsLink,
			LinkTarget:   stats.FileInfo.LinkTarget,
			IsReadable:   stats.FileInfo.IsReadable,
			IsWritable:   stats.FileInfo.IsWritable,
			IsExecutable: stats.FileInfo.IsExecutable,
			MimeType:     stats.FileInfo.MimeType,
			Children:     stats.FileInfo.Children,
			Permissions: types.FilePermission{
				User: types.PermissionBits{
					Read:    stats.FileInfo.Permissions.User.Read,
					Write:   stats.FileInfo.Permissions.User.Write,
					Execute: stats.FileInfo.Permissions.User.Execute,
				},
				Group: types.PermissionBits{
					Read:    stats.FileInfo.Permissions.Group.Read,
					Write:   stats.FileInfo.Permissions.Group.Write,
					Execute: stats.FileInfo.Permissions.Group.Execute,
				},
				Other: types.PermissionBits{
					Read:    stats.FileInfo.Permissions.Other.Read,
					Write:   stats.FileInfo.Permissions.Other.Write,
					Execute: stats.FileInfo.Permissions.Other.Execute,
				},
			},
		},
		DiskUsage:  stats.DiskUsage,
		FileCount:  stats.FileCount,
		DirCount:   stats.DirCount,
		TotalSize:  stats.TotalSize,
		Checksum:   stats.Checksum,
		LastAccess: stats.LastAccess.Format("2006-01-02T15:04:05Z07:00"),
	}

	l.Infof("成功获取文件统计信息: %s, 总大小: %d, 文件数: %d, 目录数: %d",
		req.Path, resp.TotalSize, resp.FileCount, resp.DirCount)

	return resp, nil
}
