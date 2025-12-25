package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResumeResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResumeResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResumeResourceLogic {
	return &ResumeResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResumeResourceLogic) ResumeResource(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	switch strings.ToUpper(versionDetail.ResourceType) {
	case "DEPLOYMENT":
		err = client.Deployment().ResumeRollout(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Deployment 恢复更新失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "恢复更新",
				fmt.Sprintf("Deployment %s/%s 恢复更新失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("恢复更新失败")
		}
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "恢复更新",
			fmt.Sprintf("Deployment %s/%s 恢复更新成功", versionDetail.Namespace, versionDetail.ResourceName), 1)
	case "JOB":
		err = client.Job().Resume(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("Job 恢复失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "恢复任务",
				fmt.Sprintf("Job %s/%s 恢复任务失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("恢复任务失败")
		}
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "恢复任务",
			fmt.Sprintf("Job %s/%s 恢复任务成功", versionDetail.Namespace, versionDetail.ResourceName), 1)
	case "CRONJOB":
		err = client.CronJob().Resume(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("CronJob 恢复失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "恢复调度",
				fmt.Sprintf("CronJob %s/%s 恢复调度失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
			return "", fmt.Errorf("恢复调度失败")
		}
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "恢复调度",
			fmt.Sprintf("CronJob %s/%s 恢复调度成功", versionDetail.Namespace, versionDetail.ResourceName), 1)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持恢复操作", versionDetail.ResourceType)
	}

	return "恢复成功", nil
}
