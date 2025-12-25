package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type TriggerGCLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 手动触发GC
func NewTriggerGCLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TriggerGCLogic {
	return &TriggerGCLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TriggerGCLogic) TriggerGC(req *types.TriggerGCRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.TriggerGC(l.ctx, &pb.TriggerGCReq{
		RegistryUuid:   req.RegistryUuid,
		DeleteUntagged: req.DeleteUntagged,
		Workers:        req.Workers,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("GC任务触发成功: JobId=%d", rpcResp.JobId)
	return "GC任务触发成功", nil
}
