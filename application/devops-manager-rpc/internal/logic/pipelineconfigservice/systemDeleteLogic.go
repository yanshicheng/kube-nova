package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SystemDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSystemDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemDeleteLogic {
	return &SystemDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SystemDeleteLogic) SystemDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.SystemModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("系统删除失败: %v", err)
		return nil, err
	}
	if err := ensureSystemAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("系统删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.SystemModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("系统删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
