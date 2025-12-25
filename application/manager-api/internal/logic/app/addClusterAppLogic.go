package app

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddClusterAppLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加或更新集群应用配置，如果应用已存在则更新，否则创建新应用
func NewAddClusterAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddClusterAppLogic {
	return &AddClusterAppLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddClusterAppLogic) AddClusterApp(req *types.AddClusterAppRequest) (resp string, err error) {
	rpcReq := &managerservice.AddClusterAppReq{
		ClusterUuid:        req.ClusterUuid,
		AppName:            req.AppName,
		AppCode:            req.AppCode,
		AppType:            req.AppType,
		IsDefault:          req.IsDefault,
		AppUrl:             req.AppUrl,
		Port:               req.Port,
		Protocol:           req.Protocol,
		AuthEnabled:        req.AuthEnabled,
		AuthType:           req.AuthType,
		Username:           req.Username,
		Password:           req.Password,
		Token:              req.Token,
		AccessKey:          req.AccessKey,
		AccessSecret:       req.AccessSecret,
		TlsEnabled:         req.TlsEnabled,
		CaFile:             req.CaFile,
		CaKey:              req.CaKey,
		CaCert:             req.CaCert,
		ClientCert:         req.ClientCert,
		ClientKey:          req.ClientKey,
		InsecureSkipVerify: req.InsecureSkipVerify,
		UpdatedBy:          req.UpdatedBy,
	}

	// 调用RPC服务
	_, err = l.svcCtx.ManagerRpc.AppAdd(l.ctx, rpcReq)
	if err != nil {
		l.Errorf("调用RPC服务添加/更新应用失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "集群中间件保存",
			ActionDetail: fmt.Sprintf("保存集群中间件配置失败，中间件名称：%s，类型：%s，地址：%s:%d", req.AppName, req.AppType, req.AppUrl, req.Port),
			Status:       0,
		})

		return "", fmt.Errorf("保存应用配置失败: %w", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "集群中间件保存",
		ActionDetail: fmt.Sprintf("成功保存集群中间件配置，中间件名称：%s，类型：%d，地址：%s:%d，认证类型：%s", req.AppName, req.AppType, req.AppUrl, req.Port, req.AuthType),
		Status:       1,
	})

	return "中间件保存成功", nil
}
