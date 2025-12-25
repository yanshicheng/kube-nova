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

type AlertRuleFileAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleFileAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleFileAddLogic {
	return &AlertRuleFileAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleFileAdd 新增告警规则文件
func (l *AlertRuleFileAddLogic) AlertRuleFileAdd(in *pb.AddAlertRuleFileReq) (*pb.AddAlertRuleFileResp, error) {
	// 检查 file_code 是否已存在
	existFile, err := l.svcCtx.AlertRuleFilesModel.FindOneByFileCode(l.ctx, in.FileCode)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, errorx.Msg("查询文件代码失败")
	}
	if existFile != nil {
		return nil, errorx.Msg("文件代码已存在")
	}

	// 设置默认命名空间
	namespace := in.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}

	// 插入数据
	data := &model.AlertRuleFiles{
		FileCode:    in.FileCode,
		FileName:    in.FileName,
		Description: in.Description,
		Namespace:   namespace,
		Labels:      sql.NullString{String: in.Labels, Valid: in.Labels != ""},
		IsEnabled:   boolToInt(in.IsEnabled),
		SortOrder:   in.SortOrder,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
		IsDeleted:   0,
	}

	result, err := l.svcCtx.AlertRuleFilesModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("新增告警规则文件失败")
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, errorx.Msg("获取新增ID失败")
	}

	return &pb.AddAlertRuleFileResp{
		Id: uint64(id),
	}, nil
}

// boolToInt 将 bool 转换为 int64
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
