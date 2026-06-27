package managerservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/inspection"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTaskRunLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTaskRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTaskRunLogic {
	return &InspectionTaskRunLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTaskRunLogic) InspectionTaskRun(in *pb.InspectionTaskRunReq) (*pb.InspectionTaskRunResp, error) {
	var locker incremental.DistributedLocker = incremental.NewNoopLocker("inspection-manual")
	if l.svcCtx.Cache != nil && l.svcCtx.Config.InspectionEngine.LockEnabled {
		locker = incremental.NewLockWithAutoRenew(l.svcCtx.Cache, "inspection-manual")
	}
	lockCtx := context.Background()
	acquired, release, err := locker.TryLock(lockCtx, fmt.Sprintf("cluster-inspection:manual:%d", in.Id), l.svcCtx.Config.InspectionEngine.LockTTL)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, errorx.Msg("巡检任务正在执行，请稍后查看巡检报告中心")
	}

	records, err := inspection.NewRunner(lockCtx, l.svcCtx).RunTaskAsync(in.Id, in.ClusterUuid, "manual", in.Operator, release)
	if err != nil {
		release()
		return nil, err
	}

	return &pb.InspectionTaskRunResp{Records: records}, nil
}
