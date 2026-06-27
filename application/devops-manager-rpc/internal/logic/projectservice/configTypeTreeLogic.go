package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigTypeTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfigTypeTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigTypeTreeLogic {
	return &ConfigTypeTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfigTypeTreeLogic) ConfigTypeTree(in *pb.ConfigTypeTreeReq) (*pb.ConfigTypeTreeResp, error) {
	data, err := l.svcCtx.ConfigTypeModel.All(l.ctx, in.Status)
	if err != nil {
		l.Errorf("配置类型树查询失败: %v", err)
		return nil, err
	}

	return &pb.ConfigTypeTreeResp{Data: configTypeTreeToPb(data)}, nil
}
