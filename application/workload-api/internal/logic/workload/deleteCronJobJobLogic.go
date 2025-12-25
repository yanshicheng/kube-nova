package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteCronJobJobLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 CronJob Job
func NewDeleteCronJobJobLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCronJobJobLogic {
	return &DeleteCronJobJobLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteCronJobJobLogic) DeleteCronJobJob(req *types.GetDefaultJobNameRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	jobOperator := client.Job()
	err = jobOperator.Delete(versionDetail.Namespace, req.JobName)
	if err != nil {
		l.Errorf("删除 CronJob Job 失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "删除Job",
			fmt.Sprintf("CronJob %s/%s 删除关联Job %s 失败: %v", versionDetail.Namespace, versionDetail.ResourceName, req.JobName, err), 2)
		return "", fmt.Errorf("删除 CronJob Job 失败: %v", err)
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "删除Job",
		fmt.Sprintf("CronJob %s/%s 删除关联Job %s 成功", versionDetail.Namespace, versionDetail.ResourceName, req.JobName), 1)
	return "删除 CronJob Job 成功", nil
}
