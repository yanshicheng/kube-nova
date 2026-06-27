package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigTypeGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfigTypeGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigTypeGetLogic {
	return &ConfigTypeGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfigTypeGetLogic) ConfigTypeGet(in *pb.GetByIdReq) (*pb.GetConfigTypeResp, error) {
	data, err := l.svcCtx.ConfigTypeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("配置类型查询失败: %v", err)
		return nil, err
	}

	return &pb.GetConfigTypeResp{Data: configTypeToPb(data)}, nil
}
