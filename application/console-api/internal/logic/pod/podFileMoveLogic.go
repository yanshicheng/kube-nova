package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileMoveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 移动或重命名文件
func NewPodFileMoveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileMoveLogic {
	return &PodFileMoveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileMoveLogic) PodFileMove(req *types.PodFileMoveReq) (resp *types.PodFileMoveResp, err error) {
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

	// 5. 移动文件
	err = podClient.MoveFile(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.SourcePath, req.DestPath, req.Overwrite)
	if err != nil {
		l.Errorf("移动文件失败: %v", err)
		return nil, fmt.Errorf("移动文件失败: %v", err)
	}

	// 6. 构建响应
	resp = &types.PodFileMoveResp{
		SourcePath: req.SourcePath,
		DestPath:   req.DestPath,
		Success:    true,
	}

	l.Infof("成功移动文件: %s -> %s", req.SourcePath, req.DestPath)

	return resp, nil
}
