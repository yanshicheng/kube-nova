package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppOnClusterListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAppOnClusterListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppOnClusterListLogic {
	return &AppOnClusterListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AppOnClusterList 获取指定集群的所有应用列表
func (l *AppOnClusterListLogic) AppOnClusterList(in *pb.ClusterAppListReq) (*pb.ClusterAppListResp, error) {
	if in.ClusterUuid == "" {
		l.Errorf("集群UUID不能为空")
		return nil, errorx.Msg("集群UUID不能为空")
	}

	_, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群不存在 [clusterUuid=%s]", in.ClusterUuid)
			return nil, errorx.Msg("指定的集群不存在")
		}
		l.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群信息失败")
	}

	// 3. 查询集群下的所有应用
	queryStr := "`cluster_uuid` = ?"
	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(
		l.ctx,
		"created_at", // 按创建时间排序
		false,        // 降序排列，最新的在前面
		queryStr,
		in.ClusterUuid,
	)

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("集群下暂无应用配置 [clusterUuid=%s]", in.ClusterUuid)
			// 返回空列表而不是错误
			return &pb.ClusterAppListResp{
				List: []*pb.ClusterAppDetailResp{},
			}, nil
		}
		l.Errorf("查询集群应用列表失败: %v", err)
		return nil, errorx.Msg("查询集群应用列表失败")
	}

	var appList []*pb.ClusterAppDetailResp

	for _, app := range apps {
		// 构建单个应用的详情响应
		appDetail := &pb.ClusterAppDetailResp{
			Id:                 app.Id,
			ClusterUuid:        app.ClusterUuid,
			AppName:            app.AppName,
			AppCode:            app.AppCode,
			AppType:            app.AppType,
			IsDefault:          app.IsDefault,
			AppUrl:             app.AppUrl,
			Port:               app.Port,
			Protocol:           app.Protocol,
			AuthEnabled:        app.AuthEnabled,
			AuthType:           app.AuthType,
			Username:           app.Username,
			Password:           app.Password,
			AccessKey:          app.AccessKey,
			AccessSecret:       app.AccessSecret,
			TlsEnabled:         app.TlsEnabled,
			InsecureSkipVerify: app.InsecureSkipVerify,
			Status:             app.Status,
			CreatedBy:          app.CreatedBy,
			UpdatedBy:          app.UpdatedBy,
			CreatedAt:          app.CreatedAt.Unix(),
			UpdatedAt:          app.UpdatedAt.Unix(),
		}

		// 处理可选的SQL NULL字段
		if app.Token.Valid {
			appDetail.Token = app.Token.String
		}

		if app.CaFile.Valid {
			appDetail.CaFile = app.CaFile.String
		}

		if app.CaKey.Valid {
			appDetail.CaKey = app.CaKey.String
		}

		if app.CaCert.Valid {
			appDetail.CaCert = app.CaCert.String
		}

		if app.ClientCert.Valid {
			appDetail.ClientCert = app.ClientCert.String
		}

		if app.ClientKey.Valid {
			appDetail.ClientKey = app.ClientKey.String
		}

		appList = append(appList, appDetail)
	}

	resp := &pb.ClusterAppListResp{
		List: appList,
	}
	return resp, nil
}
