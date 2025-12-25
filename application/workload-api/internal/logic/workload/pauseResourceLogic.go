package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PauseResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPauseResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PauseResourceLogic {
	return &PauseResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PauseResourceLogic) PauseResource(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	switch strings.ToUpper(versionDetail.ResourceType) {
	case "DEPLOYMENT":
		err = client.Deployment().PauseRollout(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Deployment 暂停更新失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "暂停更新",
				fmt.Sprintf("暂停更新失败: %v", err), 2)
			return "", fmt.Errorf("暂停更新失败")
		}
	case "JOB":
		err = client.Job().Suspend(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Job 暂停失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "暂停任务",
				fmt.Sprintf("暂停任务失败: %v", err), 2)
			return "", fmt.Errorf("暂停任务失败")
		}
	case "CRONJOB":
		err = client.CronJob().Suspend(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("CronJob 暂停失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "暂停调度",
				fmt.Sprintf("暂停调度失败: %v", err), 2)
			return "", fmt.Errorf("暂停调度失败")
		}
	default:
		return "", fmt.Errorf("资源类型 %s 不支持暂停操作", versionDetail.ResourceType)
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "暂停更新", "暂停成功", 1)
	return "暂停成功", nil
}
