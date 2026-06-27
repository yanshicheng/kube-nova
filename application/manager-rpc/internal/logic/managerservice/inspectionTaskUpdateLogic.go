package managerservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTaskUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTaskUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTaskUpdateLogic {
	return &InspectionTaskUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTaskUpdateLogic) InspectionTaskUpdate(in *pb.InspectionTaskUpdateReq) (*pb.InspectionTaskResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("任务ID不能为空")
	}
	item, err := l.svcCtx.OnecInspectionTaskModel.FindOne(l.ctx, in.Id)
	if err != nil || item.IsDeleted == 1 {
		return nil, errorx.Msg("巡检任务不存在")
	}
	scopeType := inspectionString(in.ScopeType, "cluster")
	if scopeType != "cluster" && scopeType != "global" {
		return nil, errorx.Msg("巡检范围仅支持 global 或 cluster")
	}
	if scopeType == "cluster" && strings.TrimSpace(in.ClusterUuid) == "" && !in.PrometheusEnabled {
		return nil, errorx.Msg("单集群巡检必须指定集群UUID")
	}
	scheduleType := inspectionString(in.ScheduleType, "manual")
	if scheduleType != "manual" && scheduleType != "cron" {
		return nil, errorx.Msg("调度类型仅支持 manual 或 cron")
	}
	if scheduleType == "cron" && strings.TrimSpace(in.CronExpr) == "" {
		return nil, errorx.Msg("定时巡检必须配置Cron表达式")
	}
	clusterUuid := strings.TrimSpace(in.ClusterUuid)
	if scopeType == "global" {
		clusterUuid = ""
	}
	item.Name = in.Name
	item.Description = in.Description
	item.TemplateId = in.TemplateId
	item.ScopeType = scopeType
	item.ClusterUuid = clusterUuid
	item.ScheduleType = scheduleType
	item.CronExpr = in.CronExpr
	item.Enabled = inspectionBool(in.Enabled)
	item.MaxConcurrency = in.MaxConcurrency
	item.TimeoutSec = in.TimeoutSec
	item.ConfigJson = inspectionNullString(in.ConfigJson)
	item.PrometheusEnabled = inspectionBool(in.PrometheusEnabled)
	item.PrometheusEndpoint = strings.TrimSpace(in.PrometheusEndpoint)
	item.PrometheusAuthEnabled = inspectionBool(in.PrometheusAuthEnabled)
	item.PrometheusAuthType = strings.TrimSpace(in.PrometheusAuthType)
	item.PrometheusUsername = strings.TrimSpace(in.PrometheusUsername)
	item.PrometheusPassword = strings.TrimSpace(in.PrometheusPassword)
	item.PrometheusToken = inspectionNullString(in.PrometheusToken)
	item.PrometheusTlsEnabled = inspectionBool(in.PrometheusTlsEnabled)
	item.PrometheusInsecureSkipVerify = inspectionBool(in.PrometheusInsecureSkipVerify)
	item.PrometheusCaCert = inspectionNullString(in.PrometheusCaCert)
	item.PrometheusClientCert = inspectionNullString(in.PrometheusClientCert)
	item.PrometheusClientKey = inspectionNullString(in.PrometheusClientKey)
	item.UpdatedBy = inspectionString(in.UpdatedBy, "system")
	if item.PrometheusEnabled == 1 && item.PrometheusEndpoint == "" {
		return nil, errorx.Msg("启用计划级 Prometheus 时访问地址不能为空")
	}
	if item.MaxConcurrency <= 0 {
		item.MaxConcurrency = 2
	}
	if item.TimeoutSec <= 0 {
		item.TimeoutSec = 600
	}
	if scheduleType == "cron" {
		item.NextRunAt = inspectionNextRun(item.CronExpr, time.Now())
	} else {
		item.NextRunAt.Valid = false
	}
	if err := l.svcCtx.OnecInspectionTaskModel.Update(l.ctx, item); err != nil {
		return nil, errorx.Msg("更新巡检任务失败")
	}

	return &pb.InspectionTaskResp{Data: inspectionTaskToPB(item)}, nil
}
