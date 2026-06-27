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

type GetLogClusterConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLogClusterConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogClusterConfigLogic {
	return &GetLogClusterConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetLogClusterConfigLogic) GetLogClusterConfig(in *pb.GetLogClusterConfigReq) (*pb.GetLogClusterConfigResp, error) {
	clusterUUID := strings.TrimSpace(in.ClusterUuid)
	if clusterUUID == "" {
		return nil, errorx.Msg("集群UUID不能为空")
	}

	if _, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUUID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("指定的集群不存在")
		}
		return nil, errorx.Msg("查询集群信息失败")
	}

	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ?", clusterUUID, 2)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.GetLogClusterConfigResp{
				ClusterUuid: clusterUUID,
				Backends:    []*pb.LogClusterConfigItem{},
			}, nil
		}
		return nil, errorx.Msg("查询集群日志配置失败")
	}

	resp := &pb.GetLogClusterConfigResp{
		ClusterUuid: clusterUUID,
		Backends:    make([]*pb.LogClusterConfigItem, 0, len(apps)),
	}
	for _, app := range apps {
		resp.Backends = append(resp.Backends, &pb.LogClusterConfigItem{
			Id:                 app.Id,
			ClusterUuid:        app.ClusterUuid,
			AppName:            app.AppName,
			AppCode:            app.AppCode,
			AppType:            app.AppType,
			IsDefault:          app.IsDefault == 1,
			AppUrl:             app.AppUrl,
			Port:               app.Port,
			Protocol:           app.Protocol,
			AuthEnabled:        app.AuthEnabled == 1,
			AuthType:           app.AuthType,
			TlsEnabled:         app.TlsEnabled == 1,
			InsecureSkipVerify: app.InsecureSkipVerify == 1,
			Status:             app.Status,
			CreatedAt:          app.CreatedAt.Unix(),
			UpdatedAt:          app.UpdatedAt.Unix(),
		})
		if app.IsDefault == 1 && resp.DefaultBackend == "" {
			resp.DefaultBackend = app.AppCode
		}
	}

	return resp, nil
}
