package managerservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleFileUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleFileUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleFileUpdateLogic {
	return &AlertRuleFileUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleFileUpdate 更新告警规则文件
func (l *AlertRuleFileUpdateLogic) AlertRuleFileUpdate(in *pb.UpdateAlertRuleFileReq) (*pb.UpdateAlertRuleFileResp, error) {
	// 检查文件是否存在
	existFile, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则文件不存在")
		}
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	// 如果修改了 file_code，需要检查新的 file_code 是否已被其他文件使用
	if in.FileCode != "" && in.FileCode != existFile.FileCode {
		checkFile, err := l.svcCtx.AlertRuleFilesModel.FindOneByFileCode(l.ctx, in.FileCode)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("查询文件代码失败")
		}
		if checkFile != nil && checkFile.Id != existFile.Id {
			return nil, errorx.Msg("文件代码已存在")
		}
	}

	// 更新数据
	existFile.FileCode = in.FileCode
	existFile.FileName = in.FileName
	existFile.Description = in.Description
	existFile.Namespace = in.Namespace
	existFile.Labels = sql.NullString{String: in.Labels, Valid: in.Labels != ""}
	existFile.IsEnabled = boolToInt(in.IsEnabled)
	existFile.SortOrder = in.SortOrder
	existFile.UpdatedBy = in.UpdatedBy

	err = l.svcCtx.AlertRuleFilesModel.Update(l.ctx, existFile)
	if err != nil {
		return nil, errorx.Msg("更新告警规则文件失败")
	}

	return &pb.UpdateAlertRuleFileResp{}, nil
}
