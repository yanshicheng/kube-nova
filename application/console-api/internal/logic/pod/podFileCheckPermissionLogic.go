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

type PodFileCheckPermissionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查文件权限
func NewPodFileCheckPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileCheckPermissionLogic {
	return &PodFileCheckPermissionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileCheckPermissionLogic) PodFileCheckPermission(req *types.PodFileCheckPermissionReq) (resp *types.PodFileCheckPermissionResp, err error) {
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

	// 5. 解析权限类型
	var permission k8stypes.FilePermission
	switch req.Permission {
	case "read":
		permission = k8stypes.FilePermission{User: k8stypes.PermissionBits{Read: true}}
	case "write":
		permission = k8stypes.FilePermission{User: k8stypes.PermissionBits{Write: true}}
	case "execute":
		permission = k8stypes.FilePermission{User: k8stypes.PermissionBits{Execute: true}}
	default:
		return nil, fmt.Errorf("无效的权限类型: %s，可选值: read/write/execute", req.Permission)
	}

	// 6. 检查权限
	hasPermission, err := podClient.CheckFilePermission(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, permission)
	if err != nil {
		l.Errorf("检查文件权限失败: %v", err)
		return nil, fmt.Errorf("检查文件权限失败: %v", err)
	}

	// 7. 获取文件信息
	fileInfo, err := podClient.GetFileInfo(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path)
	if err != nil {
		l.Errorf("获取文件信息失败: %v", err)
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 8. 构建响应
	resp = &types.PodFileCheckPermissionResp{
		HasPermission: hasPermission,
		CurrentMode:   fileInfo.Mode,
		Owner:         fileInfo.Owner,
		Group:         fileInfo.Group,
	}

	l.Infof("权限检查完成: %s, 类型: %s, 结果: %v", req.Path, req.Permission, hasPermission)

	return resp, nil
}
