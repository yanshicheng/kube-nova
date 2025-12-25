package app

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateClusterAppLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试中间件是否正常，验证应用的连通性和认证配置
func NewValidateClusterAppLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateClusterAppLogic {
	return &ValidateClusterAppLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ValidateClusterAppLogic) ValidateClusterApp(req *types.ClusterAppValidateRequest) (resp string, err error) {

	_, err = l.svcCtx.ManagerRpc.AppValidate(l.ctx, &managerservice.ClusterAppValidateReq{
		Id: uint64(req.Id),
	})
	if err != nil {
		l.Errorf("获取应用详情失败，无法进行验证，ID: %d, error: %v", req.Id, err)
		return "", fmt.Errorf("测试连接失败!")
	}

	return "测试连接成功", nil
}
