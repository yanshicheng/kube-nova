package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/operator"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestRegistryConnectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTestRegistryConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestRegistryConnectionLogic {
	return &TestRegistryConnectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TestRegistryConnectionLogic) TestRegistryConnection(in *pb.TestRegistryConnectionReq) (*pb.TestRegistryConnectionResp, error) {
	switch in.Type {
	case "harbor":
		return l.testHarborConnection(in)
	default:
		return &pb.TestRegistryConnectionResp{
			Success: false,
			Message: "不支持的仓库类型: " + in.Type,
		}, nil
	}
}

// testHarborConnection 测试 Harbor 连接
func (l *TestRegistryConnectionLogic) testHarborConnection(in *pb.TestRegistryConnectionReq) (*pb.TestRegistryConnectionResp, error) {
	config := &types.HarborConfig{
		UUID:     "connection-test",
		Name:     "connection-test",
		Endpoint: in.Url,
		Username: in.Username,
		Password: in.Password,
		Insecure: in.Insecure,
		CACert:   in.CaCert,
	}

	client, err := operator.NewHarborClient(l.ctx, config)
	if err != nil {
		l.Errorf("创建 Harbor 客户端失败: %v", err)
		return &pb.TestRegistryConnectionResp{
			Success: false,
			Message: "创建客户端失败: " + err.Error(),
		}, nil
	}

	// 确保客户端被关闭
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			l.Errorf("关闭测试客户端失败: %v", closeErr)
		}
	}()

	// 测试连接
	if err := client.Ping(); err != nil {
		l.Errorf("Harbor 连接测试失败: url=%s, error=%v", in.Url, err)
		return &pb.TestRegistryConnectionResp{
			Success: false,
			Message: "连接测试失败: " + err.Error(),
		}, nil
	}

	l.Infof("Harbor 连接测试成功: url=%s", in.Url)
	return &pb.TestRegistryConnectionResp{
		Success: true,
		Message: "连接成功",
	}, nil
}
