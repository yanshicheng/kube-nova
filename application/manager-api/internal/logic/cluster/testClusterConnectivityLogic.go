package cluster

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type TestClusterConnectivityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestClusterConnectivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestClusterConnectivityLogic {
	return &TestClusterConnectivityLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestClusterConnectivityLogic) TestClusterConnectivity(req *types.TestClusterConnectivityRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.ClusterTestConnect(l.ctx, &managerservice.ClusterTestConnectReq{
		ApiServerHost:      req.ApiServerHost,
		AuthType:           req.AuthType,
		Token:              req.Token,
		CaCert:             req.CaCert,
		CaFile:             req.CaFile,
		KubeFile:           req.KubeFile,
		KeyFile:            req.KeyFile,
		CertFile:           req.CertFile,
		ClientCert:         req.ClientCert,
		ClientKey:          req.ClientKey,
		InsecureSkipVerify: req.InsecureSkipVerify,
	})
	if err != nil {
		l.Errorf("测试集群连接失败: error=%v", err)
		return
	}
	l.Infof("测试集群连接成功")
	return "测试集群连接成功", nil
}
