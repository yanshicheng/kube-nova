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

type PodFileSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存文件内容
func NewPodFileSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileSaveLogic {
	return &PodFileSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileSaveLogic) PodFileSave(req *types.PodFileSaveReq) (resp *types.PodFileSaveResp, err error) {
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

	// 5. 构建保存选项
	saveOpts := &k8stypes.SaveOptions{
		CreateIfNotExists: req.CreateIfNotExists,
		Backup:            req.Backup,
		Encoding:          req.Encoding,
		FileMode:          req.FileMode,
	}

	// 6. 保存文件内容
	err = podClient.SaveFile(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, []byte(req.Content), saveOpts)
	if err != nil {
		l.Errorf("保存文件失败: %v", err)
		return nil, fmt.Errorf("保存文件失败: %v", err)
	}

	// 7. 获取保存后的文件信息
	fileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取保存后的文件信息失败: %v", err)
		// 即使获取信息失败，也认为保存成功
		fileInfo = &k8stypes.FileInfo{
			Path: req.Path,
			Size: int64(len(req.Content)),
		}
	}

	// 8. 构建响应
	resp = &types.PodFileSaveResp{
		FilePath: fileInfo.Path,
		FileSize: fileInfo.Size,
	}

	// 如果启用了备份，尝试获取备份文件路径
	if req.Backup {
		// 备份文件通常命名为 原文件名.bak 或 原文件名~
		resp.BackupPath = fmt.Sprintf("%s.bak", req.Path)
	}

	l.Infof("成功保存文件: %s, 大小: %d 字节, 备份: %v", req.Path, resp.FileSize, req.Backup)

	return resp, nil
}
