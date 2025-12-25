package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleFileListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleFileListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleFileListLogic {
	return &AlertRuleFileListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleFileList 无分页查询告警规则文件列表（用于下拉选择）
func (l *AlertRuleFileListLogic) AlertRuleFileList(in *pb.ListAlertRuleFileReq) (*pb.ListAlertRuleFileResp, error) {
	var queryStr string
	var args []interface{}

	if in.Namespace != "" {
		queryStr = "`namespace` = ?"
		args = append(args, in.Namespace)
	}

	list, err := l.svcCtx.AlertRuleFilesModel.SearchNoPage(
		l.ctx,
		"sort_order",
		true,
		queryStr,
		args...,
	)
	if err != nil {
		return nil, errorx.Msg("查询告警规则文件列表失败")
	}

	var files []*pb.AlertRuleFile
	for _, item := range list {
		files = append(files, &pb.AlertRuleFile{
			Id:        item.Id,
			FileCode:  item.FileCode,
			FileName:  item.FileName,
			Namespace: item.Namespace,
			IsEnabled: item.IsEnabled == 1,
			SortOrder: item.SortOrder,
		})
	}

	return &pb.ListAlertRuleFileResp{
		Data: files,
	}, nil
}
