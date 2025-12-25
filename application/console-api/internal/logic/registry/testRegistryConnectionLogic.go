package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type TestRegistryConnectionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试仓库连接
func NewTestRegistryConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestRegistryConnectionLogic {
	return &TestRegistryConnectionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestRegistryConnectionLogic) TestRegistryConnection(req *types.TestRegistryConnectionRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.TestRegistryConnection(l.ctx, &pb.TestRegistryConnectionReq{
		Url:      req.Url,
		Username: req.Username,
		Password: req.Password,
		Insecure: req.Insecure,
		CaCert:   req.CaCert,
		Type:     req.Type,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("测试仓库连接完成: Success=%v, Message=%s", rpcResp.Success, rpcResp.Message)
	return "测试仓库连接完成", nil
}
