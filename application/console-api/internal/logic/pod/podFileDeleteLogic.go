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

type PodFileDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除文件或目录
func NewPodFileDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileDeleteLogic {
	return &PodFileDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileDeleteLogic) PodFileDelete(req *types.PodFileDeleteReq) (resp *types.PodFileDeleteResp, err error) {
	// 1. 验证参数
	if len(req.Paths) == 0 {
		return nil, fmt.Errorf("删除路径列表不能为空")
	}

	// 2. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 3. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 4. 初始化 Pod 操作器
	podClient := client.Pods()

	// 5. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
	}

	// 6. 构建删除选项
	deleteOpts := &k8stypes.DeleteOptions{
		Recursive: req.Recursive,
		Force:     req.Force,
	}

	// 7. 删除文件
	result, err := podClient.DeleteFiles(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Paths, deleteOpts)
	if err != nil {
		l.Errorf("删除文件失败: %v", err)
		return nil, fmt.Errorf("删除文件失败: %v", err)
	}

	// 8. 转换错误信息
	errorMessages := make([]string, 0, len(result.Errors))
	for _, e := range result.Errors {
		if e != nil {
			errorMessages = append(errorMessages, e.Error())
		}
	}

	// 9. 构建响应
	resp = &types.PodFileDeleteResp{
		DeletedCount: result.DeletedCount,
		FailedPaths:  result.FailedPaths,
		Errors:       errorMessages,
	}

	l.Infof("删除操作完成，成功: %d, 失败: %d", resp.DeletedCount, len(resp.FailedPaths))

	return resp, nil
}
