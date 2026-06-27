package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigTypeDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfigTypeDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigTypeDeleteLogic {
	return &ConfigTypeDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfigTypeDeleteLogic) ConfigTypeDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.ConfigTypeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("配置类型删除失败: %v", err)
		return nil, err
	}
	if err := ensureConfigTypeDeletable(l.ctx, l.svcCtx, data); err != nil {
		l.Errorf("配置类型删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ConfigTypeModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("配置类型删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
