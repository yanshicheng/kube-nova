package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTemplateAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTemplateAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTemplateAddLogic {
	return &InspectionTemplateAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------自动化集群巡检-----------------------
func (l *InspectionTemplateAddLogic) InspectionTemplateAdd(in *pb.InspectionTemplateAddReq) (*pb.InspectionTemplateResp, error) {
	if in.Name == "" || in.Code == "" {
		return nil, errorx.Msg("模板名称和编码不能为空")
	}
	item := &model.OnecInspectionTemplate{
		Name:        in.Name,
		Code:        in.Code,
		Description: in.Description,
		ScopeType:   inspectionString(in.ScopeType, "cluster"),
		Enabled:     inspectionBool(in.Enabled),
		IsBuiltin:   0,
		Version:     1,
		ConfigJson:  inspectionNullString(in.ConfigJson),
		CreatedBy:   inspectionString(in.CreatedBy, "system"),
		UpdatedBy:   inspectionString(in.CreatedBy, "system"),
	}
	result, err := l.svcCtx.OnecInspectionTemplateModel.Insert(l.ctx, item)
	if err != nil {
		return nil, errorx.Msg("创建巡检模板失败")
	}
	id, _ := result.LastInsertId()
	item.Id = uint64(id)

	return &pb.InspectionTemplateResp{Data: inspectionTemplateToPB(item)}, nil
}
