package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileCopyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 复制文件或目录
func NewPodFileCopyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileCopyLogic {
	return &PodFileCopyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileCopyLogic) PodFileCopy(req *types.PodFileCopyReq) (resp *types.PodFileCopyResp, err error) {
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

	// 5. 构建复制选项
	copyOpts := &k8stypes.CopyOptions{
		Overwrite:     req.Overwrite,
		Recursive:     req.Recursive,
		PreserveAttrs: req.PreserveAttrs,
	}

	// 6. 复制文件
	err = podClient.CopyFile(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.SourcePath, req.DestPath, copyOpts)
	if err != nil {
		l.Errorf("复制文件失败: %v", err)
		return nil, fmt.Errorf("复制文件失败: %v", err)
	}

	// 7. 获取目标文件信息以统计大小和数量
	destInfo, err := podClient.GetFileStats(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.DestPath)
	if err != nil {
		l.Errorf("获取目标文件信息失败: %v", err)
		// 使用默认值
		destInfo = &k8stypes.FileStats{
			FileCount: 1,
			TotalSize: 0,
		}
	}

	// 8. 构建响应
	resp = &types.PodFileCopyResp{
		SourcePath:  req.SourcePath,
		DestPath:    req.DestPath,
		FilesCopied: destInfo.FileCount,
		TotalSize:   destInfo.TotalSize,
	}

	l.Infof("成功复制文件: %s -> %s, 文件数: %d, 大小: %d",
		req.SourcePath, req.DestPath, resp.FilesCopied, resp.TotalSize)

	return resp, nil
}
