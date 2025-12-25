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

type AlertInstancesDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertInstancesDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertInstancesDelLogic {
	return &AlertInstancesDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertInstancesDelLogic) AlertInstancesDel(in *pb.DelAlertInstancesReq) (*pb.DelAlertInstancesResp, error) {
	// 检查告警实例是否存在
	_, err := l.svcCtx.AlertInstancesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警实例不存在")
		}
		return nil, errorx.Msg("查询告警实例失败")
	}

	// 强制删除
	err = l.svcCtx.AlertInstancesModel.Delete(l.ctx, in.Id)
	if err != nil {
		return nil, errorx.Msg("删除告警实例失败")
	}

	return &pb.DelAlertInstancesResp{}, nil
}
