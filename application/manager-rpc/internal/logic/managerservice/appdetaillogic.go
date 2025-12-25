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

type AppDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAppDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppDetailLogic {
	return &AppDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AppDetail 获取应用详情信息
func (l *AppDetailLogic) AppDetail(in *pb.ClusterAppDetailReq) (*pb.ClusterAppDetailResp, error) {
	if in.Id == 0 {
		l.Errorf("应用ID不能为空")
		return nil, errorx.Msg("应用ID不能为空")
	}

	app, err := l.svcCtx.OnecClusterAppModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用不存在 [appId=%d]", in.Id)
			return nil, errorx.Msg("指定的应用不存在")
		}
		l.Errorf("查询应用失败: %v", err)
		return nil, errorx.Msg("查询应用信息失败")
	}

	resp := &pb.ClusterAppDetailResp{
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
		resp.Token = app.Token.String
		l.Infof("包含Token信息")
	}

	if app.CaFile.Valid {
		resp.CaFile = app.CaFile.String
		l.Infof("包含CA文件信息")
	}

	if app.CaKey.Valid {
		resp.CaKey = app.CaKey.String
		l.Infof("包含CA密钥信息")
	}

	if app.CaCert.Valid {
		resp.CaCert = app.CaCert.String
		l.Infof("包含CA证书信息")
	}

	if app.ClientCert.Valid {
		resp.ClientCert = app.ClientCert.String
		l.Infof("包含客户端证书信息")
	}

	if app.ClientKey.Valid {
		resp.ClientKey = app.ClientKey.String
		l.Infof("包含客户端密钥信息")
	}

	return resp, nil
}
