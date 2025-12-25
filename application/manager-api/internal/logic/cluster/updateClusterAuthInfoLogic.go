package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateClusterAuthInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateClusterAuthInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateClusterAuthInfoLogic {
	return &UpdateClusterAuthInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateClusterAuthInfoLogic) UpdateClusterAuthInfo(req *types.UpdateClusterAuthInfoRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	l.Infof("用户 [%s] 开始更新集群认证信息 [ID=%d]", username, req.Id)

	// 先获取集群详情用于审计日志
	clusterResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return "", err
	}

	_, err = l.svcCtx.ManagerRpc.ClusterUpdateAuthInfo(l.ctx, &managerservice.ClusterUpdateAuthInfoReq{
		Id:                 req.Id,
		AuthType:           req.AuthType,
		ApiServerHost:      req.ApiServerHost,
		KubeFile:           req.KubeFile,
		Token:              req.Token,
		CaCert:             req.CaCert,
		CaFile:             req.CaFile,
		ClientCert:         req.ClientCert,
		CertFile:           req.CertFile,
		ClientKey:          req.ClientKey,
		KeyFile:            req.KeyFile,
		InsecureSkipVerify: req.InsecureSkipVerify,
	})
	if err != nil {
		l.Errorf("更新集群认证信息失败: error=%v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  clusterResp.Uuid,
			Title:        "更新集群认证信息",
			ActionDetail: fmt.Sprintf("更新集群认证信息失败，集群名称：%s，认证类型：%s，API地址：%s", clusterResp.Name, req.AuthType, req.ApiServerHost),
			Status:       0,
		})

		return "", err
	}

	// 记录成功的审计日志
	insecureStr := "否"
	if req.InsecureSkipVerify == 1 {
		insecureStr = "是"
	}
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterResp.Uuid,
		Title:        "更新集群认证信息",
		ActionDetail: fmt.Sprintf("成功更新集群认证信息，集群名称：%s，认证类型：%s，API地址：%s，跳过TLS验证：%s", clusterResp.Name, req.AuthType, req.ApiServerHost, insecureStr),
		Status:       1,
	})

	return "认证信息更新成功!", nil
}
