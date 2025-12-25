package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StopResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 停止资源
func NewStopResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StopResourceLogic {
	return &StopResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StopResourceLogic) StopResource(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = client.Deployment().Stop(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Deployment 服务停止失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "停止服务",
				fmt.Sprintf("Deployment %s/%s 停止失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务停止失败")
		}
	case "DAEMONSET":
		err = client.DaemonSet().Stop(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("DaemonSet 服务停止失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "停止服务",
				fmt.Sprintf("DaemonSet %s/%s 停止失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务停止失败")
		}
	case "CRONJOB":
		err = client.CronJob().Stop(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("CronJob 服务停止失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "停止服务",
				fmt.Sprintf("CronJob %s/%s 停止失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务停止失败")
		}
	case "STATEFULSET":
		err = client.StatefulSet().Stop(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("StatefulSet 服务停止失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "停止服务",
				fmt.Sprintf("StatefulSet %s/%s 停止失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("服务停止失败")
		}
	default:
		return "", fmt.Errorf("不支持的资源类型")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "停止服务",
		fmt.Sprintf("%s %s/%s 停止成功", resourceType, versionDetail.Namespace, versionDetail.ResourceName), 1)
	return "停止完成", nil
}
