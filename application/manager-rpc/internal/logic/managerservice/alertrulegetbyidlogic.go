package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleGetByIdLogic {
	return &AlertRuleGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleGetById 根据ID查询告警规则
func (l *AlertRuleGetByIdLogic) AlertRuleGetById(in *pb.GetAlertRuleByIdReq) (*pb.GetAlertRuleByIdResp, error) {
	data, err := l.svcCtx.AlertRulesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则不存在")
		}
		return nil, errorx.Msg("查询告警规则失败")
	}

	// 获取所属分组信息
	var groupCode, groupName string
	var fileId uint64
	var fileCode string
	group, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, data.GroupId)
	if err == nil {
		groupCode = group.GroupCode
		groupName = group.GroupName
		fileId = group.FileId

		// 获取所属文件信息
		file, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, group.FileId)
		if err == nil {
			fileCode = file.FileCode
		}
	}

	return &pb.GetAlertRuleByIdResp{
		Data: &pb.AlertRule{
			Id:          data.Id,
			GroupId:     data.GroupId,
			AlertName:   data.AlertName,
			RuleNameCn:  data.RuleNameCn,
			Expr:        data.Expr,
			ForDuration: data.ForDuration,
			Severity:    data.Severity,
			Summary:     data.Summary,
			Description: data.Description.String,
			Labels:      data.Labels.String,
			Annotations: data.Annotations.String,
			IsEnabled:   data.IsEnabled == 1,
			SortOrder:   data.SortOrder,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
			GroupCode:   groupCode,
			GroupName:   groupName,
			FileId:      fileId,
			FileCode:    fileCode,
		},
	}, nil
}
