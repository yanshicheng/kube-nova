package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertGroupAppsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchAlertGroupAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertGroupAppsLogic {
	return &SearchAlertGroupAppsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertGroupAppsLogic) SearchAlertGroupApps(req *types.SearchAlertGroupAppsRequest) (resp *types.SearchAlertGroupAppsResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("搜索告警组应用请求: operator=%s, groupId=%d, appType=%s",
		username, req.GroupId, req.AppType)

	// 调用 RPC 服务搜索告警组应用
	result, err := l.svcCtx.AlertPortalRpc.SearchAlertGroupApps(l.ctx, &pb.SearchAlertGroupAppsReq{
		GroupId: req.GroupId,
		AppType: req.AppType,
	})
	if err != nil {
		l.Errorf("搜索告警组应用失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换数据
	items := make([]types.AlertGroupApps, 0)

	// RPC 返回的是单个对象，需要判断是否有数据
	if result.Id != 0 {
		items = append(items, types.AlertGroupApps{
			Id:        result.Id,
			GroupId:   result.GroupId,
			AppId:     result.AppId,
			AppType:   result.AppType,
			CreatedBy: result.CreatedBy,
			UpdatedBy: result.UpdatedBy,
		})
	}

	resp = &types.SearchAlertGroupAppsResponse{
		Items: items,
	}

	l.Infof("搜索告警组应用成功: operator=%s, count=%d", username, len(items))
	return resp, nil
}
