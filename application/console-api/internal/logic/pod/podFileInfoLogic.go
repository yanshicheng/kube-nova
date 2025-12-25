package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取文件详细信息
func NewPodFileInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileInfoLogic {
	return &PodFileInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileInfoLogic) PodFileInfo(req *types.PodFileInfoReq) (resp *types.FileInfo, err error) {
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

	// 5. 获取文件信息
	fileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取文件信息失败: %v", err)
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 6. 转换为响应格式
	resp = &types.FileInfo{
		Name:         fileInfo.Name,
		Path:         fileInfo.Path,
		IsDir:        fileInfo.IsDir,
		Size:         fileInfo.Size,
		Mode:         fileInfo.Mode,
		ModTime:      fileInfo.ModTime.Format("2006-01-02T15:04:05Z07:00"),
		Owner:        fileInfo.Owner,
		Group:        fileInfo.Group,
		IsLink:       fileInfo.IsLink,
		LinkTarget:   fileInfo.LinkTarget,
		IsReadable:   fileInfo.IsReadable,
		IsWritable:   fileInfo.IsWritable,
		IsExecutable: fileInfo.IsExecutable,
		MimeType:     fileInfo.MimeType,
		Children:     fileInfo.Children,
		Permissions: types.FilePermission{
			User: types.PermissionBits{
				Read:    fileInfo.Permissions.User.Read,
				Write:   fileInfo.Permissions.User.Write,
				Execute: fileInfo.Permissions.User.Execute,
			},
			Group: types.PermissionBits{
				Read:    fileInfo.Permissions.Group.Read,
				Write:   fileInfo.Permissions.Group.Write,
				Execute: fileInfo.Permissions.Group.Execute,
			},
			Other: types.PermissionBits{
				Read:    fileInfo.Permissions.Other.Read,
				Write:   fileInfo.Permissions.Other.Write,
				Execute: fileInfo.Permissions.Other.Execute,
			},
		},
	}

	l.Infof("成功获取文件信息: %s, 大小: %d, 类型: %s", req.Path, resp.Size, resp.Mode)

	return resp, nil
}
