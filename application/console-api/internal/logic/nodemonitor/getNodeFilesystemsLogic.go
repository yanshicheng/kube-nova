package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeFilesystemsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点文件系统列表
func NewGetNodeFilesystemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeFilesystemsLogic {
	return &GetNodeFilesystemsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeFilesystemsLogic) GetNodeFilesystems(req *types.GetNodeFilesystemsRequest) (resp *types.GetNodeFilesystemsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	filesystems, err := node.GetNodeFilesystems(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点文件系统列表失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeFilesystemsResponse{}
	for _, fs := range filesystems {
		resp.Data = append(resp.Data, convertNodeFilesystemMetrics(&fs))
	}

	l.Infof("获取节点文件系统列表成功: Node=%s, Count=%d", req.NodeName, len(filesystems))
	return resp, nil
}
