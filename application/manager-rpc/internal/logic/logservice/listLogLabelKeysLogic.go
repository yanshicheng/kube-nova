package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogLabelKeysLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLogLabelKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogLabelKeysLogic {
	return &ListLogLabelKeysLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListLogLabelKeysLogic) ListLogLabelKeys(in *pb.ListLogLabelKeysReq) (*pb.ListLogLabelKeysResp, error) {
	keys, err := listLogLabelKeys(in.SourceType, in.QueryMode)
	if err != nil {
		return nil, err
	}
	return &pb.ListLogLabelKeysResp{Keys: keys}, nil
}
