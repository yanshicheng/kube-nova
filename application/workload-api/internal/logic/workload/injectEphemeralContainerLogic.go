package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type InjectEphemeralContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 注入临时容器
func NewInjectEphemeralContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InjectEphemeralContainerLogic {
	return &InjectEphemeralContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InjectEphemeralContainerLogic) InjectEphemeralContainer(req *types.InjectEphemeralContainerRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
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
		Namespace:     versionDetail.Namespace,
		ContainerName: containerName,
		Image:         image,
		Args:          args,
		Command:       command,
	})
	if err != nil {
		l.Errorf("Pod 注入临时容器失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "注入临时容器",
			fmt.Sprintf("资源 %s/%s 的Pod %s 注入临时容器失败, 容器名: %s, 镜像: %s, 错误: %v", versionDetail.Namespace, versionDetail.ResourceName, req.PodName, containerName, image, err), 2)
		return "", fmt.Errorf("注入临时容器失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "注入临时容器",
		fmt.Sprintf("资源 %s/%s 的Pod %s 注入临时容器成功, 容器名: %s, 镜像: %s", versionDetail.Namespace, versionDetail.ResourceName, req.PodName, containerName, image), 1)
	return "注入成功", nil
}
