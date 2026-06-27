package logservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogQueryModeOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLogQueryModeOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogQueryModeOptionsLogic {
	return &GetLogQueryModeOptionsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetLogQueryModeOptionsLogic) GetLogQueryModeOptions(in *pb.LogQueryModeOptionsReq) (*pb.LogQueryModeOptionsResp, error) {
	if strings.TrimSpace(in.ClusterUuid) == "" {
		return nil, errorx.Msg("集群UUID不能为空")
	}

	if _, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.ClusterUuid); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("指定的集群不存在")
		}
		return nil, errorx.Msg("查询集群信息失败")
	}

	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ?", in.ClusterUuid, 2)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.LogQueryModeOptionsResp{}, nil
		}
		return nil, errorx.Msg("查询集群应用列表失败")
	}

	resp := &pb.LogQueryModeOptionsResp{}
	for _, app := range apps {
		code := strings.ToLower(strings.TrimSpace(app.AppCode))
		if strings.Contains(code, "loki") {
			resp.Loki = true
		}
		if strings.Contains(code, "elasticsearch") || strings.Contains(code, "elastic") || code == "es" {
			resp.Elasticsearch = true
		}
	}

	return resp, nil
}
