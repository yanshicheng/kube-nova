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

type AlertRuleGroupAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleGroupAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleGroupAddLogic {
	return &AlertRuleGroupAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleGroupAdd 新增告警规则分组
func (l *AlertRuleGroupAddLogic) AlertRuleGroupAdd(in *pb.AddAlertRuleGroupReq) (*pb.AddAlertRuleGroupResp, error) {
	// 检查所属文件是否存在
	_, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, in.FileId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("所属告警规则文件不存在")
		}
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	// 检查 file_id + group_code 唯一性
	existGroup, err := l.svcCtx.AlertRuleGroupsModel.FindOneByFileIdGroupCode(l.ctx, in.FileId, in.GroupCode)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, errorx.Msg("查询分组代码失败")
	}
	if existGroup != nil {
		return nil, errorx.Msg("该文件下分组代码已存在")
	}

	// 设置默认间隔
	interval := in.Interval
	if interval == "" {
		interval = "30s"
	}

	// 插入数据
	data := &model.AlertRuleGroups{
		FileId:      in.FileId,
		GroupCode:   in.GroupCode,
		GroupName:   in.GroupName,
		Description: in.Description,
		Interval:    interval,
		IsEnabled:   boolToInt(in.IsEnabled),
		SortOrder:   in.SortOrder,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
		IsDeleted:   0,
	}

	result, err := l.svcCtx.AlertRuleGroupsModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("新增告警规则分组失败")
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, errorx.Msg("获取新增ID失败")
	}

	return &pb.AddAlertRuleGroupResp{
		Id: uint64(id),
	}, nil
}
