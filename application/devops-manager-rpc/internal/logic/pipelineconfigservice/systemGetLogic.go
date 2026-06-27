package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SystemGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSystemGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemGetLogic {
	return &SystemGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SystemGetLogic) SystemGet(in *pb.GetByIdReq) (*pb.GetSystemResp, error) {
	data, err := l.svcCtx.SystemModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("系统查询详情失败: %v", err)
		return nil, err
	}
	if err := ensureSystemAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("系统查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetSystemResp{Data: systemToPb(data)}, nil
}
