package workload

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

func recordAuditLog(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	versionDetail *managerservice.GetOnecProjectVersionDetailResp,
	title string,
	actionDetail string,
	status int64,
) {
	if versionDetail == nil {
		return
	}

	_, auditErr := svcCtx.ManagerRpc.ProjectAuditLogAdd(ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        title,
		ActionDetail: actionDetail,
		Status:       status,
	})

	if auditErr != nil {
		logx.Errorf("记录审计日志失败: %v", auditErr)
	}
}

func recordAuditLogByClusterUuid(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	clusterUuid string,
	title string,
	actionDetail string,
	status int64,
) {
	_, auditErr := svcCtx.ManagerRpc.ProjectAuditLogAdd(ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterUuid,
		Title:        title,
		ActionDetail: actionDetail,
		Status:       status,
	})

	if auditErr != nil {
		logx.Errorf("记录审计日志失败: %v", auditErr)
	}
}
