package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionGroupDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionGroupDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionGroupDelLogic {
	return &InspectionGroupDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionGroupDelLogic) InspectionGroupDel(in *pb.InspectionGroupDelReq) (*pb.DeleteClusterResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("巡检分组ID不能为空")
	}
	item, err := l.svcCtx.OnecInspectionGroupModel.FindOne(l.ctx, in.Id)
	if err != nil || item.IsDeleted == 1 {
		return nil, errorx.Msg("巡检分组不存在")
	}
	items, err := l.svcCtx.OnecInspectionItemModel.SearchNoPage(l.ctx, "id", true, "`group_id` = ?", in.Id)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("检查巡检分组规则失败")
	}
	for _, rule := range items {
		if rule != nil && rule.IsDeleted == 0 {
			return nil, errorx.Msg("巡检分组下存在规则，请先删除或迁移规则")
		}
	}
	if err := l.svcCtx.OnecInspectionGroupModel.DeleteSoft(l.ctx, in.Id); err != nil {
		return nil, errorx.Msg("删除巡检分组失败")
	}

	return &pb.DeleteClusterResp{}, nil
}
