package managerservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTaskAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTaskAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTaskAddLogic {
	return &InspectionTaskAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTaskAddLogic) InspectionTaskAdd(in *pb.InspectionTaskAddReq) (*pb.InspectionTaskResp, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errorx.Msg("任务名称不能为空")
	}
	scopeType := inspectionString(in.ScopeType, "cluster")
	if scopeType != "cluster" && scopeType != "global" {
		return nil, errorx.Msg("巡检范围仅支持 global 或 cluster")
	}
	if scopeType == "cluster" && strings.TrimSpace(in.ClusterUuid) == "" {
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
	item := &model.OnecInspectionTask{
		Name:                         in.Name,
		Description:                  in.Description,
		TemplateId:                   in.TemplateId,
		ScopeType:                    scopeType,
		ClusterUuid:                  clusterUuid,
		ScheduleType:                 scheduleType,
		CronExpr:                     in.CronExpr,
		Enabled:                      inspectionBool(in.Enabled),
		MaxConcurrency:               in.MaxConcurrency,
		TimeoutSec:                   in.TimeoutSec,
		LastStatus:                   "pending",
		ConfigJson:                   inspectionNullString(in.ConfigJson),
		PrometheusEnabled:            inspectionBool(in.PrometheusEnabled),
		PrometheusEndpoint:           strings.TrimSpace(in.PrometheusEndpoint),
		PrometheusAuthEnabled:        inspectionBool(in.PrometheusAuthEnabled),
		PrometheusAuthType:           strings.TrimSpace(in.PrometheusAuthType),
		PrometheusUsername:           strings.TrimSpace(in.PrometheusUsername),
		PrometheusPassword:           strings.TrimSpace(in.PrometheusPassword),
		PrometheusToken:              inspectionNullString(in.PrometheusToken),
		PrometheusTlsEnabled:         inspectionBool(in.PrometheusTlsEnabled),
		PrometheusInsecureSkipVerify: inspectionBool(in.PrometheusInsecureSkipVerify),
		PrometheusCaCert:             inspectionNullString(in.PrometheusCaCert),
		PrometheusClientCert:         inspectionNullString(in.PrometheusClientCert),
		PrometheusClientKey:          inspectionNullString(in.PrometheusClientKey),
		CreatedBy:                    inspectionString(in.CreatedBy, "system"),
		UpdatedBy:                    inspectionString(in.CreatedBy, "system"),
	}
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
	}
	result, err := l.svcCtx.OnecInspectionTaskModel.Insert(l.ctx, item)
	if err != nil {
		return nil, errorx.Msg("创建巡检任务失败")
	}
	id, _ := result.LastInsertId()
	item.Id = uint64(id)

	return &pb.InspectionTaskResp{Data: inspectionTaskToPB(item)}, nil
}
