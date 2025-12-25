package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 读取文件内容
func NewPodFileReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileReadLogic {
	return &PodFileReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileReadLogic) PodFileRead(req *types.PodFileReadReq) (resp *types.PodFileReadResp, err error) {
	// 1. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
	}

	// 5. 构建读取选项
	readOpts := &k8stypes.ReadOptions{
		Offset:    req.Offset,
		Limit:     req.Limit,
		Encoding:  req.Encoding,
		Tail:      req.Tail,
		TailLines: req.TailLines,
	}

	// 6. 读取文件内容
	content, err := podClient.ReadFile(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, req.Path, readOpts)
	if err != nil {
		l.Errorf("读取文件失败: %v", err)
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	// 7. 转换为响应格式
	resp = &types.PodFileReadResp{
		Content:     content.Content,
		FileSize:    content.FileSize,
		BytesRead:   content.BytesRead,
		IsText:      content.IsText,
		Encoding:    content.Encoding,
		IsTruncated: content.IsTruncated,
		LineCount:   content.LineCount,
	}

	l.Infof("成功读取文件: %s, 文件大小: %d, 读取字节: %d, 是否文本: %v",
		req.Path, resp.FileSize, resp.BytesRead, resp.IsText)

	return resp, nil
}
