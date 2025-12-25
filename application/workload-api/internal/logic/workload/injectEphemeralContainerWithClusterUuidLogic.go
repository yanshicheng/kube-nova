package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type InjectEphemeralContainerWithClusterUuidLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通过 clusteruuid 获取 注入临时容器
func NewInjectEphemeralContainerWithClusterUuidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InjectEphemeralContainerWithClusterUuidLogic {
	return &InjectEphemeralContainerWithClusterUuidLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InjectEphemeralContainerWithClusterUuidLogic) InjectEphemeralContainerWithClusterUuid(req *types.InjectEphemeralContainerWithClusterUuidRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		logx.Errorf("集群管理器获取失败: %s", err.Error())
		return "", fmt.Errorf("集群管理器获取失败: %s", err.Error())
	}
	containerName := req.ContainerName
	if containerName == "" {
		containerName = "ephemeral-default0"
	}
	image := req.Image
	args := req.Args
	command := req.Command
	if image == "" {
		image = l.svcCtx.Config.InjectImage
		command = []string{"sleep", "infinity"}
		args = []string{}
	}
	err = client.Pods().InjectEphemeralContainer(&types2.InjectEphemeralContainerRequest{
		PodName:       req.PodName,
		Namespace:     req.Namespace,
		ContainerName: containerName,
		Image:         image,
		Args:          args,
		Command:       command,
	})
	if err != nil {
		l.Errorf("Pod 注入临时容器失败: %v", err)
		recordAuditLogByClusterUuid(l.ctx, l.svcCtx, req.ClusterUuid, "注入临时容器",
			fmt.Sprintf("Pod %s/%s 注入临时容器失败, 容器名: %s, 镜像: %s, 错误: %v", req.Namespace, req.PodName, containerName, image, err), 2)
		return "", fmt.Errorf("注入临时容器失败")
	}

	recordAuditLogByClusterUuid(l.ctx, l.svcCtx, req.ClusterUuid, "注入临时容器",
		fmt.Sprintf("Pod %s/%s 注入临时容器成功, 容器名: %s, 镜像: %s", req.Namespace, req.PodName, containerName, image), 1)
	return "注入成功", nil
}
