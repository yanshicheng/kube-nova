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

type PodFileChmodLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改文件权限
func NewPodFileChmodLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileChmodLogic {
	return &PodFileChmodLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileChmodLogic) PodFileChmod(req *types.PodFileChmodReq) (resp *types.PodFileChmodResp, err error) {
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

	// 5. 获取修改前的文件信息
	fileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取文件信息失败: %v", err)
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}
	oldMode := fileInfo.Mode

	// 6. 修改权限（通过 exec 执行 chmod 命令）
	chmodCmd := []string{"chmod"}
	if req.Recursive {
		chmodCmd = append(chmodCmd, "-R")
	}
	chmodCmd = append(chmodCmd, req.Mode, req.Path)

	_, stderr, execErr := podClient.ExecCommand(workloadInfo.Data.Namespace, req.PodName, containerName, chmodCmd)
	if execErr != nil {
		l.Errorf("修改文件权限失败: %v, stderr: %s", execErr, stderr)
		return nil, fmt.Errorf("修改文件权限失败: %v", execErr)
	}

	// 7. 获取修改后的文件信息
	newFileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取修改后的文件信息失败: %v", err)
		newFileInfo = &k8stypes.FileInfo{Mode: req.Mode}
	}

	// 8. 构建响应
	resp = &types.PodFileChmodResp{
		Path:    req.Path,
		OldMode: oldMode,
		NewMode: newFileInfo.Mode,
		Success: true,
	}

	l.Infof("成功修改文件权限: %s, %s -> %s", req.Path, oldMode, newFileInfo.Mode)

	return resp, nil
}
