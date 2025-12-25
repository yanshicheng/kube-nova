package managerservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAppAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppAddLogic {
	return &AppAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AppAdd 添加或更新集群应用配置 (upsert操作)
func (l *AppAddLogic) AppAdd(in *pb.AddClusterAppReq) (*pb.AddClusterAppResp, error) {
	// 1. 验证集群是否存在
	l.Infof("步骤1: 验证集群存在性 [clusterUuid=%s]", in.ClusterUuid)
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群不存在 [clusterUuid=%s]", in.ClusterUuid)
			return nil, errorx.Msg("指定的集群不存在")
		}
		l.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群信息失败")
	}
	l.Infof("集群验证成功 [clusterName=%s]", cluster.Name)

	// 2. 检查应用是否已存在（根据集群UUID、应用代码和应用类型）
	l.Infof("步骤2: 检查应用是否已存在")
	existingApp, err := l.svcCtx.OnecClusterAppModel.FindOneByClusterUuidAppCodeAppType(
		l.ctx, in.ClusterUuid, in.AppCode, in.AppType)

	var isUpdate bool
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询现有应用失败: %v", err)
		return nil, errorx.Msg("查询应用信息失败")
	}

	if existingApp != nil {
		isUpdate = true
		l.Infof("发现已存在的应用，将执行更新操作 [appId=%d, appName=%s]",
			existingApp.Id, existingApp.AppName)
	} else {
		l.Infof("应用不存在，将执行创建操作")
	}

	// 3. 检查默认应用的唯一性约束
	if in.IsDefault == 1 {
		l.Infof("步骤3: 检查默认应用唯一性约束")
		queryStr := "`cluster_uuid` = ? AND `app_type` = ? AND `is_default` = 1"
		defaultApps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(
			l.ctx, "", true, queryStr, in.ClusterUuid, in.AppType)

		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询默认应用失败: %v", err)
			return nil, errorx.Msg("查询默认应用失败")
		}

		// 检查是否存在其他默认应用
		for _, defaultApp := range defaultApps {
			// 如果是更新操作，排除当前正在更新的应用
			if isUpdate && defaultApp.Id == existingApp.Id {
				continue
			}

			// 发现其他默认应用，返回错误
			l.Errorf("集群中已存在该类型的默认应用 [existingAppId=%d, existingAppName=%s]",
				defaultApp.Id, defaultApp.AppName)
			return nil, errorx.Msg(fmt.Sprintf("集群中已存在类型为%d的默认应用: %s，每种类型只能设置一个默认应用",
				in.AppType, defaultApp.AppName))
		}

		if len(defaultApps) > 0 && !isUpdate {
			l.Infof("发现%d个默认应用，但都不冲突", len(defaultApps))
		}
		l.Infof("默认应用唯一性验证通过")
	}

	// 4. 构建应用数据模型
	l.Infof("步骤4: 构建应用数据模型")
	var app *model.OnecClusterApp

	if isUpdate {
		// 更新现有应用
		app = existingApp
	} else {
		// 创建新应用
		app = &model.OnecClusterApp{}
	}

	// 更新应用基本信息
	app.ClusterUuid = in.ClusterUuid
	app.AppName = in.AppName
	app.AppCode = in.AppCode
	app.AppType = in.AppType
	app.IsDefault = in.IsDefault
	app.AppUrl = in.AppUrl
	app.Port = in.Port
	app.Protocol = in.Protocol
	app.AuthEnabled = in.AuthEnabled
	app.AuthType = in.AuthType
	app.Username = in.Username
	app.Password = in.Password
	app.AccessKey = in.AccessKey
	app.AccessSecret = in.AccessSecret
	app.TlsEnabled = in.TlsEnabled
	app.InsecureSkipVerify = in.InsecureSkipVerify
	app.Status = 1 // 默认设置为正常状态
	app.UpdatedBy = in.UpdatedBy

	if !isUpdate {
		app.CreatedBy = in.UpdatedBy
	}

	// 处理可选的SQL NULL字段
	if in.Token != "" {
		app.Token = sql.NullString{String: in.Token, Valid: true}
	} else {
		app.Token = sql.NullString{Valid: false}
	}

	if in.CaFile != "" {
		app.CaFile = sql.NullString{String: in.CaFile, Valid: true}
	} else {
		app.CaFile = sql.NullString{Valid: false}
	}

	if in.CaKey != "" {
		app.CaKey = sql.NullString{String: in.CaKey, Valid: true}
	} else {
		app.CaKey = sql.NullString{Valid: false}
	}

	if in.CaCert != "" {
		app.CaCert = sql.NullString{String: in.CaCert, Valid: true}
	} else {
		app.CaCert = sql.NullString{Valid: false}
	}

	if in.ClientCert != "" {
		app.ClientCert = sql.NullString{String: in.ClientCert, Valid: true}
	} else {
		app.ClientCert = sql.NullString{Valid: false}
	}

	if in.ClientKey != "" {
		app.ClientKey = sql.NullString{String: in.ClientKey, Valid: true}
	} else {
		app.ClientKey = sql.NullString{Valid: false}
	}

	// 5. 执行数据库操作
	l.Infof("步骤5: 执行数据库操作")
	if isUpdate {
		// 更新现有应用
		err = l.svcCtx.OnecClusterAppModel.Update(l.ctx, app)
		if err != nil {
			l.Errorf("更新应用失败: %v", err)
			return nil, errorx.Msg("更新应用配置失败")
		}
		l.Infof("应用更新成功 [appId=%d]", app.Id)
	} else {
		// 创建新应用
		_, err = l.svcCtx.OnecClusterAppModel.Insert(l.ctx, app)
		if err != nil {
			l.Errorf("创建应用失败: %v", err)
			return nil, errorx.Msg("创建应用配置失败")
		}
		l.Infof("应用创建成功")
	}

	return &pb.AddClusterAppResp{}, nil
}
