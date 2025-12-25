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

type ApplicationDelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除应用
func NewApplicationDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationDelLogic {
	return &ApplicationDelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationDelLogic) ApplicationDel(req *types.DefaultIdRequest) (resp string, err error) {
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	// 查询该应用下的所有版本
	versions, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: req.Id,
	})
	if err != nil {
		l.Errorf("查询应用版本失败: %v", err)
		return "", err
	}

	// 删除所有版本及其 K8s 资源
	for _, version := range versions.Data {
		// 获取版本详情
		versionDetail, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
			Id: version.Id,
		})
		if err != nil {
			l.Errorf("获取版本详情失败: %v", err)
			continue
		}

		// 删除 K8s 资源
		if err := l.deleteK8sResource(versionDetail); err != nil {
			l.Errorf("删除 K8s 资源失败: %v", err)
			// 继续删除其他资源
		}

		// 删除版本记录
		_, err = l.svcCtx.ManagerRpc.VersionDel(l.ctx, &managerservice.DelOnecProjectVersionReq{
			Id: version.Id,
		})
		if err != nil {
			l.Errorf("删除版本记录失败: %v", err)
		}
	}

	// 删除应用记录
	_, err = l.svcCtx.ManagerRpc.ApplicationDel(l.ctx, &managerservice.DelOnecProjectApplicationReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除应用失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ApplicationId: req.Id,
			Title:         "删除应用",
			ActionDetail:  fmt.Sprintf("删除应用失败, 错误: %v", err),
			Status:        0,
		})

		return "", err
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ApplicationId: req.Id,
		Title:         "删除应用",
		ActionDetail:  "删除应用及所有版本",
		Status:        1,
	})

	return "删除成功", nil
}

func (l *ApplicationDelLogic) deleteK8sResource(detail *managerservice.GetOnecProjectVersionDetailResp) error {
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
