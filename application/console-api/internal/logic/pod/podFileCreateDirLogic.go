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

type PodFileCreateDirLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建目录
func NewPodFileCreateDirLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileCreateDirLogic {
	return &PodFileCreateDirLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileCreateDirLogic) PodFileCreateDir(req *types.PodFileCreateDirReq) (resp *types.PodFileCreateDirResp, err error) {
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

	// 5. 构建创建目录选项
	createOpts := &k8stypes.CreateDirOptions{
		Mode:          req.Mode,
		CreateParents: req.CreateParents,
	}

	// 6. 创建目录
	err = podClient.CreateDirectory(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, createOpts)
	if err != nil {
		l.Errorf("创建目录失败: %v", err)
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	// 7. 构建响应
	resp = &types.PodFileCreateDirResp{
		Path:    req.Path,
		Created: true,
	}

	l.Infof("成功创建目录: %s, 权限: %s", req.Path, req.Mode)

	return resp, nil
}
