package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type StartResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启动资源
func NewStartResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartResourceLogic {
	return &StartResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StartResourceLogic) StartResource(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().Start(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Deployment 服务启动失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "启动服务",
				fmt.Sprintf("Deployment %s/%s 启动失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务启动失败")
		}
	case "DAEMONSET":
		err = client.DaemonSet().Start(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("DaemonSet 服务启动失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "启动服务",
				fmt.Sprintf("DaemonSet %s/%s 启动失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务启动失败")
		}
	case "CRONJOB":
		err = client.CronJob().Start(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("CronJob 服务启动失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "启动服务",
				fmt.Sprintf("CronJob %s/%s 启动失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务启动失败")
		}
	case "STATEFULSET":
		err = client.StatefulSet().Start(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("StatefulSet 服务启动失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "启动服务",
				fmt.Sprintf("StatefulSet %s/%s 启动失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务启动失败")
		}
	default:
		return "", fmt.Errorf("不支持的资源类型")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "启动服务",
		fmt.Sprintf("%s %s/%s 启动成功", resourceType, versionDetail.Namespace, versionDetail.ResourceName), 1)
	return "启动完成", nil
}
