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

type AlertRuleFileDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleFileDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleFileDelLogic {
	return &AlertRuleFileDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleFileDel 删除告警规则文件（支持级联删除）
func (l *AlertRuleFileDelLogic) AlertRuleFileDel(in *pb.DelAlertRuleFileReq) (*pb.DelAlertRuleFileResp, error) {
	// 检查文件是否存在
	_, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则文件不存在")
		}
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	// 查询该文件下的所有分组
	groups, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "", true, "`file_id` = ?", in.Id)
	if err != nil {
		l.Logger.Errorf("查询关联分组失败: %v", err)
	}

	if len(groups) > 0 {
		if !in.Cascade {
			return nil, errorx.Msg("该文件下存在分组，请先删除分组或选择级联删除")
		}

		// 级联删除：先删除所有分组下的规则，再删除分组
		for _, group := range groups {
			// 删除该分组下的所有规则
			err = l.svcCtx.AlertRulesModel.DeleteSoftByGroupId(l.ctx, group.Id)
			if err != nil {
				l.Logger.Errorf("删除分组 %d 下的规则失败: %v", group.Id, err)
			}

			// 删除分组
			err = l.svcCtx.AlertRuleGroupsModel.DeleteSoft(l.ctx, group.Id)
			if err != nil {
				l.Logger.Errorf("删除分组 %d 失败: %v", group.Id, err)
			}
		}
	}

	// 软删除文件
	err = l.svcCtx.AlertRuleFilesModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("删除告警规则文件失败")
	}

	return &pb.DelAlertRuleFileResp{}, nil
}
