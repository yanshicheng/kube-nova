package managerservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAuditLogDelBeforeDaysLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAuditLogDelBeforeDaysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAuditLogDelBeforeDaysLogic {
	return &ProjectAuditLogDelBeforeDaysLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectAuditLogDelBeforeDays 删除指定天数之前的审计日志数据
func (l *ProjectAuditLogDelBeforeDaysLogic) ProjectAuditLogDelBeforeDays(in *pb.DelOnecProjectAuditLogBeforeDaysReq) (*pb.DelOnecProjectAuditLogBeforeDaysResp, error) {
	// 参数校验
	if in.Days == 0 {
		l.Errorf("参数校验失败: days 不能为0")
		return nil, errorx.Msg("删除天数不能为0")
	}

	// 限制删除范围，防止误删除（至少保留1天的数据）
	if in.Days < 1 {
		l.Errorf("参数校验失败: days 必须大于等于1, days=%d", in.Days)
		return nil, errorx.Msg("删除天数必须大于等于1")
	}

	// 计算截止时间
	cutoffTime := time.Now().AddDate(0, 0, -int(in.Days))
	cutoffTimeStr := cutoffTime.Format("2006-01-02 15:04:05")

	l.Infof("开始删除 %d 天前的审计日志数据，截止时间: %s", in.Days, cutoffTimeStr)

	// 先查询要删除的数据，获取所有ID用于清理缓存
	queryStr := "`created_at` < ?"
	args := []interface{}{cutoffTime}

	// 查询要删除的记录（不分页，获取所有符合条件的记录）
	records, err := l.svcCtx.OnecProjectAuditLog.SearchNoPage(
		l.ctx,
		"created_at",
		true,
		queryStr,
		args...,
	)

	if err != nil {
		l.Errorf("查询待删除的审计日志失败: %v", err)
		return nil, errorx.Msg("查询待删除的审计日志失败")
	}

	if len(records) == 0 {
		l.Infof("没有找到需要删除的审计日志数据")
		return &pb.DelOnecProjectAuditLogBeforeDaysResp{}, nil
	}

	for _, record := range records {
		// 删除数据
		err := l.svcCtx.OnecProjectAuditLog.Delete(l.ctx, record.Id)
		if err != nil {
			l.Errorf("删除审计日志失败: %v, id=%d", err, record.Id)
			return nil, err
		}
	}
	return &pb.DelOnecProjectAuditLogBeforeDaysResp{}, nil
}
