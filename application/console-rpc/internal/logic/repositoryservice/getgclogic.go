package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGCLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGCLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGCLogic {
	return &GetGCLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGCLogic) GetGC(in *pb.GetGCReq) (*pb.GetGCResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	gc, err := client.GC().Get(in.GcId)
	if err != nil {
		return nil, errorx.Msg("获取GC详情失败")
	}

	return &pb.GetGCResp{
		Data: &pb.GCHistory{
			Id:             gc.ID,
			JobName:        gc.JobName,
			JobKind:        gc.JobKind,
			JobStatus:      gc.JobStatus,
			CreationTime:   gc.CreationTime.Unix(),
			UpdateTime:     gc.UpdateTime.Unix(),
			DeleteUntagged: gc.DeleteUntagged,
			JobParameters:  gc.JobParameters,
		},
	}, nil
}
