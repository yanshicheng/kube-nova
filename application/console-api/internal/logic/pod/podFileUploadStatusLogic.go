package pod

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileUploadStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPodFileUploadStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileUploadStatusLogic {
	return &PodFileUploadStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileUploadStatusLogic) PodFileUploadStatus(req *types.PodFileUploadStatusReq) (resp *types.PodFileUploadStatusResp, err error) {
	// 通过 ServiceContext 调用通用上传服务
	session, uploadedChunks, missingChunks, err := l.svcCtx.UploadCoreClient.GetStatus(req.SessionId)
	if err != nil {
		return nil, err
	}

	progress := float64(len(uploadedChunks)) / float64(session.TotalChunks) * 100

	resp = &types.PodFileUploadStatusResp{
		SessionId:      session.SessionID,
		FileName:       session.FileName,
		FileSize:       session.FileSize,
		TotalChunks:    session.TotalChunks,
		UploadedChunks: uploadedChunks,
		MissingChunks:  missingChunks,
		Progress:       progress,
		Status:         session.Status,
		ExpiresAt:      session.ExpiresAt.Unix(),
		CreatedAt:      session.CreatedAt.Unix(),
	}

	return resp, nil
}
