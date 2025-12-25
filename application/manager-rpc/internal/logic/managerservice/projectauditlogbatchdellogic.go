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

type ProjectAuditLogBatchDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAuditLogBatchDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAuditLogBatchDelLogic {
	return &ProjectAuditLogBatchDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectAuditLogBatchDel 批量删除审计日志
func (l *ProjectAuditLogBatchDelLogic) ProjectAuditLogBatchDel(in *pb.BatchDelOnecProjectAuditLogReq) (*pb.BatchDelOnecProjectAuditLogResp, error) {
	// 参数校验
	if len(in.Ids) == 0 {
		l.Errorf("参数校验失败: ids 列表不能为空")
		return nil, errorx.Msg("删除的审计日志ID列表不能为空")
	}

	// 限制批量删除数量，防止一次删除过多
	if len(in.Ids) > 100 {
		l.Errorf("参数校验失败: 批量删除数量超过限制, count=%d", len(in.Ids))
		return nil, errorx.Msg("批量删除数量不能超过100条")
	}

	var successCount int
	var failedIds []uint64

	// 批量删除
	for _, id := range in.Ids {
		if id == 0 {
			l.Errorf("跳过无效的ID: 0")
			failedIds = append(failedIds, id)
			continue
		}

		// 先查询审计日志是否存在
		_, err := l.svcCtx.OnecProjectAuditLog.FindOne(l.ctx, id)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("审计日志不存在，跳过删除: id=%d", id)
				failedIds = append(failedIds, id)
				continue
			}
			l.Errorf("查询审计日志失败: %v, id=%d", err, id)
			failedIds = append(failedIds, id)
			continue
		}

		// 执行删除（Delete 方法会自动清理缓存）
		err = l.svcCtx.OnecProjectAuditLog.Delete(l.ctx, id)
		if err != nil {
			l.Errorf("删除审计日志失败: %v, id=%d", err, id)
			failedIds = append(failedIds, id)
			continue
		}

		successCount++
		l.Infof("删除审计日志成功: id=%d", id)
	}

	// 记录批量删除结果
	if len(failedIds) > 0 {
		l.Errorf("批量删除完成，部分失败: total=%d, success=%d, failed=%d, failedIds=%v",
			len(in.Ids), successCount, len(failedIds), failedIds)
	} else {
		l.Infof("批量删除成功: total=%d, success=%d", len(in.Ids), successCount)
	}

	// 如果所有删除都失败，返回错误
	if successCount == 0 {
		return nil, errorx.Msg("批量删除失败，没有成功删除任何记录")
	}

	return &pb.BatchDelOnecProjectAuditLogResp{}, nil
}
