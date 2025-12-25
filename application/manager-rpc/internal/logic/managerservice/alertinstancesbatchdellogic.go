package managerservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertInstancesBatchDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertInstancesBatchDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertInstancesBatchDelLogic {
	return &AlertInstancesBatchDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertInstancesBatchDelLogic) AlertInstancesBatchDel(in *pb.BatchDelAlertInstancesReq) (*pb.BatchDelAlertInstancesResp, error) {
	if len(in.Ids) == 0 {
		return nil, errorx.Msg("请选择要删除的告警实例")
	}

	var failedIds []uint64
	var successCount int

	for _, id := range in.Ids {
		// 检查告警实例是否存在
		_, err := l.svcCtx.AlertInstancesModel.FindOne(l.ctx, id)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("告警实例不存在: %d", id)
				failedIds = append(failedIds, id)
				continue
			}
			l.Errorf("查询告警实例失败: %d, %v", id, err)
			failedIds = append(failedIds, id)
			continue
		}

		// 强制删除
		err = l.svcCtx.AlertInstancesModel.Delete(l.ctx, id)
		if err != nil {
			l.Errorf("删除告警实例失败: %d, %v", id, err)
			failedIds = append(failedIds, id)
			continue
		}

		successCount++
	}

	if len(failedIds) > 0 {
		return nil, errorx.Msg(fmt.Sprintf("批量删除完成，成功: %d，失败: %d (ID: %v)", successCount, len(failedIds), failedIds))
	}

	return &pb.BatchDelAlertInstancesResp{}, nil
}
