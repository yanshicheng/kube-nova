package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取podyaml
func NewGetPodYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodYamlLogic {
	return &GetPodYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodYamlLogic) GetPodYaml(req *types.GetDefaultPodNameRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}
	resp, err = client.Pods().GetYaml(versionDetail.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取PodYaml失败: %v", err)
		return "", fmt.Errorf("获取PodYaml失败")
	}
	return
}
