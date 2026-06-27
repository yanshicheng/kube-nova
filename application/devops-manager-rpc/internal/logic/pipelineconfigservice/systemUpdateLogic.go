package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SystemUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSystemUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemUpdateLogic {
	return &SystemUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SystemUpdateLogic) SystemUpdate(in *pb.UpdateSystemReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.SystemModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("系统更新失败: %v", err)
		return nil, err
	}
	if err := ensureSystemAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("系统更新失败: %v", err)
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		l.Errorf("系统名称不能为空")
		return nil, errorx.Msg("系统名称不能为空")
	}
	data.Name = strings.TrimSpace(in.Name)
	data.Description = in.Description
	data.OwnerUserIDs = in.OwnerUserIds
	data.Status = in.Status
	data.UpdatedBy = in.UpdatedBy
	if err := l.svcCtx.SystemModel.Update(l.ctx, data); err != nil {
		l.Errorf("系统更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
