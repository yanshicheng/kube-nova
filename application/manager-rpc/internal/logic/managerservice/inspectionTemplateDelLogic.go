package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTemplateDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTemplateDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTemplateDelLogic {
	return &InspectionTemplateDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTemplateDelLogic) InspectionTemplateDel(in *pb.InspectionTemplateDelReq) (*pb.DeleteClusterResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("模板ID不能为空")
	}
	item, err := l.svcCtx.OnecInspectionTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil || item.IsDeleted == 1 {
		return nil, errorx.Msg("巡检模板不存在")
	}
	if item.IsBuiltin == 1 {
		return nil, errorx.Msg("内置模板不允许删除")
	}
	if err := l.svcCtx.OnecInspectionTemplateModel.DeleteSoft(l.ctx, in.Id); err != nil {
		return nil, errorx.Msg("删除巡检模板失败")
	}

	return &pb.DeleteClusterResp{}, nil
}
