package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type VersionDelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除版本
func NewVersionDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionDelLogic {
	return &VersionDelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VersionDelLogic) VersionDel(req *types.DefaultIdRequest) (resp string, err error) {
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	// 获取版本详情
	versionDetail, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取版本详情失败: %v", err)
		return "", err
	}

	// 删除 K8s 资源
	if err := l.deleteK8sResource(versionDetail); err != nil {
		l.Errorf("删除 K8s 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "版本删除",
			ActionDetail: fmt.Sprintf("删除版本失败: %s, 错误: %v", versionDetail.ResourceName, err),
			Status:       0,
		})

		return "", fmt.Errorf("删除 K8s 资源失败: %v", err)
	}

	// 删除版本记录
	_, err = l.svcCtx.ManagerRpc.VersionDel(l.ctx, &managerservice.DelOnecProjectVersionReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除版本记录失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "版本删除",
			ActionDetail: fmt.Sprintf("删除版本记录失败: %s, 错误: %v", versionDetail.ResourceName, err),
			Status:       0,
		})

		return "", err
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "版本删除",
		ActionDetail: fmt.Sprintf("删除版本: %s", versionDetail.ResourceName),
		Status:       1,
	})

	return "删除成功", nil
}

func (l *VersionDelLogic) deleteK8sResource(detail *managerservice.GetOnecProjectVersionDetailResp) error {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, detail.ClusterUuid)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}

	resourceType := strings.ToUpper(detail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		return client.Deployment().Delete(detail.Namespace, detail.ResourceName)
	case "STATEFULSET":
		return client.StatefulSet().Delete(detail.Namespace, detail.ResourceName)
	case "DAEMONSET":
		return client.DaemonSet().Delete(detail.Namespace, detail.ResourceName)
	case "JOB":
		return client.Job().Delete(detail.Namespace, detail.ResourceName)
	case "CRONJOB":
		return client.CronJob().Delete(detail.Namespace, detail.ResourceName)
	case "POD":
		return client.Pods().Delete(detail.Namespace, detail.ResourceName)
	default:
		return fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}
