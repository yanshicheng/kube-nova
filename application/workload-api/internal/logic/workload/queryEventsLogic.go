package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询资源事件
func NewQueryEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryEventsLogic {
	return &QueryEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QueryEventsLogic) QueryEvents(req *types.EventsQueryRequest) (resp *types.EventsListResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 构建查询请求
	queryReq := types2.EventsQueryRequest{
		Namespace:          versionDetail.Namespace,
		InvolvedObjectKind: versionDetail.ResourceType,
		InvolvedObjectName: versionDetail.ResourceName,
		Type:               req.Type,
		Page:               req.Page,
		PageSize:           req.PageSize,
	}

	// 如果请求中指定了 namespace，则使用请求的 namespace
	if req.Namespace != "" {
		queryReq.Namespace = req.Namespace
	}

	// 调用 Events 操作器查询事件
	eventOperator := client.Events()
	eventsResp, err := eventOperator.QueryEvents(queryReq)
	if err != nil {
		l.Errorf("查询事件失败: %v", err)
		return nil, fmt.Errorf("查询事件失败")
	}

	resp = &types.EventsListResponse{
		Items:      convertToEventInfoList(eventsResp.Items),
		Page:       eventsResp.Page,
		PageSize:   eventsResp.PageSize,
		Total:      eventsResp.Total,
		TotalPages: eventsResp.TotalPages,
	}

	l.Infof("成功查询事件，命名空间: %s, 总数: %d", queryReq.Namespace, len(resp.Items))
	return resp, nil
}

// convertToEventInfoList 转换事件列表
func convertToEventInfoList(events []types2.EventInfo) []types.EventInfow {
	result := make([]types.EventInfow, 0, len(events))
	for _, event := range events {
		result = append(result, convertToEventInfo(event))
	}
	return result
}

// convertToEventInfo 转换单个事件信息
func convertToEventInfo(event types2.EventInfo) types.EventInfow {
	result := types.EventInfow{
		Type:               event.Type,
		Reason:             event.Reason,
		Message:            event.Message,
		Count:              event.Count,
		FirstTimestamp:     event.FirstTimestamp,
		LastTimestamp:      event.LastTimestamp,
		Source:             event.Source,
		InvolvedObjectKind: event.InvolvedObjectKind,
		InvolvedObjectName: event.InvolvedObjectName,
		InvolvedObjectUid:  event.InvolvedObjectUID,
		ReportingComponent: event.ReportingComponent,
		ReportingInstance:  event.ReportingInstance,
		Action:             event.Action,
		EventTime:          event.EventTime,
	}

	return result
}
