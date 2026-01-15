package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetKubeNovaPlatformUrlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetKubeNovaPlatformUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetKubeNovaPlatformUrlLogic {
	return &GetKubeNovaPlatformUrlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取平台地址
func (l *GetKubeNovaPlatformUrlLogic) GetKubeNovaPlatformUrl(in *pb.GetPlatformUrlReq) (*pb.GetPlatformUrlResp, error) {
	webUrl := l.svcCtx.Config.PortalUrl
	if webUrl == "" {
		webUrl = "https://www.ikubeops.com"
	}

	return &pb.GetPlatformUrlResp{
		Url: webUrl,
	}, nil
}
