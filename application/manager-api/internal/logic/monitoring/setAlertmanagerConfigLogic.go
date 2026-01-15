// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package monitoring

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetAlertmanagerConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 配置指定集群的Alertmanager
func NewSetAlertmanagerConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetAlertmanagerConfigLogic {
	return &SetAlertmanagerConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetAlertmanagerConfigLogic) SetAlertmanagerConfig(req *types.SetAlertmanagerConfigRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 设置默认值
	namespace := req.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}
	crdType := req.CrdType
	if crdType == "" {
		crdType = "secret"
	}
	name := req.Name
	if name == "" {
		name = "alertmanager-main"
	}

	// 调用RPC服务配置Alertmanager
	_, err = l.svcCtx.ManagerRpc.SetAlertmanagerConfig(l.ctx, &pb.SetAlertmanagerConfigReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   namespace,
		CrdType:     crdType,
		Name:        name,
		Operator:    username,
		Token:       l.svcCtx.Config.Webhook.Token,
	})
	if err != nil {
		l.Errorf("配置Alertmanager失败: %v", err)
		return "", err
	}

	l.Infof("用户 [%s] 成功配置集群 [%s] 的Alertmanager", username, req.ClusterUuid)
	return "Alertmanager配置成功", nil
}
