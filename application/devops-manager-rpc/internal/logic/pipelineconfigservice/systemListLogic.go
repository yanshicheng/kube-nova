package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SystemListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSystemListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemListLogic {
	return &SystemListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SystemListLogic) SystemList(in *pb.ListSystemReq) (*pb.ListSystemResp, error) {
	projectIDs, restricted, err := userProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("系统查询列表失败: %v", err)
		return nil, err
	}
	items, total, err := l.svcCtx.SystemModel.List(l.ctx, model.DevopsSystemListFilter{
		ProjectID:  in.ProjectId,
		ProjectIDs: projectIDs,
		Restricted: restricted,
		Name:       in.Name,
		Code:       in.Code,
		Status:     in.Status,
		Page:       in.Page,
		PageSize:   in.PageSize,
	})
	if err != nil {
		l.Errorf("系统查询列表失败: %v", err)
		return nil, err
	}
	data := make([]*pb.DevopsSystem, 0, len(items))
	for _, item := range items {
		data = append(data, systemToPb(item))
	}

	return &pb.ListSystemResp{Data: data, Total: total}, nil
}
