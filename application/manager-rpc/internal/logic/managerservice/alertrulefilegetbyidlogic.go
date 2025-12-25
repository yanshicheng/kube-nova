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

type AlertRuleFileGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleFileGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleFileGetByIdLogic {
	return &AlertRuleFileGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleFileGetById 根据ID查询告警规则文件
func (l *AlertRuleFileGetByIdLogic) AlertRuleFileGetById(in *pb.GetAlertRuleFileByIdReq) (*pb.GetAlertRuleFileByIdResp, error) {
	data, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则文件不存在")
		}
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	// 统计关联的分组数量
	groupCount, err := l.svcCtx.AlertRuleGroupsModel.CountByFileId(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("统计分组数量失败: %v", err)
	}

	// 统计关联的规则数量
	ruleCount, err := l.svcCtx.AlertRulesModel.CountByFileId(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("统计规则数量失败: %v", err)
	}

	return &pb.GetAlertRuleFileByIdResp{
		Data: &pb.AlertRuleFile{
			Id:          data.Id,
			FileCode:    data.FileCode,
			FileName:    data.FileName,
			Description: data.Description,
			Namespace:   data.Namespace,
			Labels:      data.Labels.String,
			IsEnabled:   data.IsEnabled == 1,
			SortOrder:   data.SortOrder,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
			GroupCount:  groupCount,
			RuleCount:   ruleCount,
		},
	}, nil
}
