// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunInspectionTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 立即执行巡检
func NewRunInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunInspectionTaskLogic {
	return &RunInspectionTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RunInspectionTaskLogic) RunInspectionTask(req *types.InspectionTaskRunRequest) (resp *types.InspectionTaskRunResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionTaskRun(l.ctx, &pb.InspectionTaskRunReq{
		Id:          req.Id,
		ClusterUuid: req.ClusterUuid,
		Operator:    currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionTaskRunResponse{Records: make([]types.InspectionRecord, 0, len(rpcResp.Records))}
	for _, item := range rpcResp.Records {
		if converted := convertRecord(item); converted != nil {
			resp.Records = append(resp.Records, *converted)
		}
	}

	return resp, nil
}
