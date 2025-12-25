package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取命名空间事件
func NewGetNamespaceEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceEventsLogic {
	return &GetNamespaceEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceEventsLogic) GetNamespaceEvents(req *types.EventsQueryRequest) (resp []types.EventInfow, err error) {
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

	// 调用 Events 操作器获取命名空间事件
	eventOperator := client.Events()
	eventsResp, err := eventOperator.GetNamespaceEvents(versionDetail.Namespace, page, pageSize)
	if err != nil {
		l.Errorf("获取命名空间事件失败: %v", err)
		return nil, fmt.Errorf("获取命名空间事件失败")
	}

	// 转换事件列表
	resp = convertToEventInfoList(eventsResp.Items)

	l.Infof("成功获取命名空间事件，命名空间: %s, 总数: %d, 页码: %d, 每页大小: %d",
		versionDetail.Namespace, len(resp), page, pageSize)
	return resp, nil
}
