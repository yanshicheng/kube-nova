package storageservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetStorageUrlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStorageUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStorageUrlLogic {
	return &GetStorageUrlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取存储的访问 url
func (l *GetStorageUrlLogic) GetStorageUrl(in *pb.GetStorageUrlRequest) (*pb.GetStorageUrlResponse, error) {

	absIcon := fmt.Sprintf("%s/%s", l.svcCtx.Config.StorageConf.EndpointProxy, l.svcCtx.Config.StorageConf.BucketName)

	return &pb.GetStorageUrlResponse{
		Url: absIcon,
	}, nil
}
