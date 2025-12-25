package pod

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileUploadChunkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPodFileUploadChunkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileUploadChunkLogic {
	return &PodFileUploadChunkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileUploadChunkLogic) PodFileUploadChunk(req *types.PodFileUploadChunkReq) (resp *types.PodFileUploadChunkResp, err error) {
	// 解码块数据
	chunkData, err := base64.StdEncoding.DecodeString(req.ChunkData)
	if err != nil {
		return nil, fmt.Errorf("解码块数据失败: %v", err)
	}

	// 验证校验和
	if req.Checksum != "" {
		actualChecksum := fmt.Sprintf("%x", md5.Sum(chunkData))
		if actualChecksum != req.Checksum {
			return nil, fmt.Errorf("块校验和不匹配")
		}
	}

	// 通过 ServiceContext 调用通用上传服务
	err = l.svcCtx.UploadCoreClient.WriteChunk(req.SessionId, req.ChunkIndex, chunkData)
	if err != nil {
		return nil, err
	}

	// 获取进度
	session, uploadedChunks, _, err := l.svcCtx.UploadCoreClient.GetStatus(req.SessionId)
	if err != nil {
		return nil, err
	}

	progress := float64(len(uploadedChunks)) / float64(session.TotalChunks) * 100

	resp = &types.PodFileUploadChunkResp{
		ChunkIndex: req.ChunkIndex,
		Uploaded:   true,
		Progress:   progress,
	}

	return resp, nil
}
