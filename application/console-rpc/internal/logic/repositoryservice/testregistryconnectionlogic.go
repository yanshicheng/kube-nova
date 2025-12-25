package repositoryservicelogic

import (
	"context"

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
	// 根据类型创建临时客户端
	if in.Type == "harbor" {
		config := &types.HarborConfig{
			UUID:     "test",
			Name:     "test",
			Endpoint: in.Url,
			Username: in.Username,
			Password: in.Password,
			Insecure: in.Insecure,
			CACert:   in.CaCert,
		}

		err := l.svcCtx.HarborManager.Add(config)
		if err != nil {
			return &pb.TestRegistryConnectionResp{
				Success: false,
				Message: "连接失败: " + err.Error(),
			}, nil
		}
		client, err := l.svcCtx.HarborManager.Get("test")
		if err != nil {
			return &pb.TestRegistryConnectionResp{
				Success: false,
				Message: "连接失败: " + err.Error(),
			}, nil
		}
		defer func(client types.HarborClient) {
			err := client.Close()
			if err != nil {
				l.Errorf("关闭连接失败: %v", err)
			}
		}(client)

		if err := client.Ping(); err != nil {
			return &pb.TestRegistryConnectionResp{
				Success: false,
				Message: "连接测试失败: " + err.Error(),
			}, nil
		}

		return &pb.TestRegistryConnectionResp{
			Success: true,
			Message: "连接成功",
		}, nil
	}

	return &pb.TestRegistryConnectionResp{
		Success: false,
		Message: "不支持的仓库类型",
	}, nil
}
