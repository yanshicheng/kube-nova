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

type AlertRuleGroupGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleGroupGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleGroupGetByIdLogic {
	return &AlertRuleGroupGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleGroupGetById 根据ID查询告警规则分组
func (l *AlertRuleGroupGetByIdLogic) AlertRuleGroupGetById(in *pb.GetAlertRuleGroupByIdReq) (*pb.GetAlertRuleGroupByIdResp, error) {
	data, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则分组不存在")
		}
		return nil, errorx.Msg("查询告警规则分组失败")
	}

	// 获取所属文件信息
	var fileCode, fileName string
	file, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, data.FileId)
	if err == nil {
		fileCode = file.FileCode
		fileName = file.FileName
	}

	// 统计关联的规则数量
	ruleCount, err := l.svcCtx.AlertRulesModel.CountByGroupId(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("统计规则数量失败: %v", err)
	}

	return &pb.GetAlertRuleGroupByIdResp{
		Data: &pb.AlertRuleGroup{
			Id:          data.Id,
			FileId:      data.FileId,
			GroupCode:   data.GroupCode,
			GroupName:   data.GroupName,
			Description: data.Description,
			Interval:    data.Interval,
			IsEnabled:   data.IsEnabled == 1,
			SortOrder:   data.SortOrder,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
			RuleCount:   ruleCount,
			FileCode:    fileCode,
			FileName:    fileName,
		},
	}, nil
}
