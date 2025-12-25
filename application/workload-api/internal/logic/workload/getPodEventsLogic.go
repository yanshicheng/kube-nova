package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取Pod事件
func NewGetPodEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodEventsLogic {
	return &GetPodEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodEventsLogic) GetPodEvents(req *types.GetPodEventsRequest) (resp []types.EventInfow, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 设置默认分页参数
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	// 调用 Events 操作器获取 Pod 事件
	eventOperator := client.Events()
	eventsResp, err := eventOperator.GetPodEvents(versionDetail.Namespace, req.PodName, page, pageSize)
	if err != nil {
		l.Errorf("获取Pod事件失败: %v", err)
		return nil, fmt.Errorf("获取Pod事件失败")
	}

	// 转换事件列表
	resp = convertToEventInfoList(eventsResp.Items)

	l.Infof("成功获取Pod事件，命名空间: %s, Pod: %s, 总数: %d, 页码: %d, 每页大小: %d",
		versionDetail.Namespace, req.PodName, len(resp), page, pageSize)
	return resp, nil
}
