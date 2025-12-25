package pod

import (
	"context"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileUploadInitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPodFileUploadInitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileUploadInitLogic {
	return &PodFileUploadInitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileUploadInitLogic) PodFileUploadInit(req *types.PodFileUploadInitReq) (resp *types.PodFileUploadInitResp, err error) {
	sessionID := uuid.New().String()
	destPath := filepath.Join(req.Path, req.FileName)

	// 构建元数据
	metadata := map[string]interface{}{
		"workloadId": float64(req.WorkloadId),
		"podName":    req.PodName,
		"container":  req.Container,
		"destPath":   destPath,
		"fileMode":   req.FileMode,
	}

	// 通过 ServiceContext 调用通用上传服务
	session, err := l.svcCtx.UploadCoreClient.Init(
		sessionID,
		"pod_file",
		req.FileName,
		req.FileSize,
		req.ChunkSize,
		10*1024*1024*1024, // 10GB
		metadata,
	)
	if err != nil {
		l.Errorf("初始化上传失败: %v", err)
		return nil, err
	}

	resp = &types.PodFileUploadInitResp{
		SessionId:   sessionID,
		ChunkSize:   session.ChunkSize,
		TotalChunks: session.TotalChunks,
		ExpiresAt:   session.ExpiresAt.Unix(),
	}

	l.Infof("初始化Pod文件上传: sessionID=%s, file=%s", sessionID, req.FileName)
	return resp, nil
}
