package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineArtifactUrlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineArtifactUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineArtifactUrlLogic {
	return &PipelineArtifactUrlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineArtifactUrlLogic) PipelineArtifactUrl(in *pb.PipelineArtifactUrlReq) (*pb.PipelineArtifactUrlResp, error) {
	objectKey := cleanArtifactObjectKey(in.ObjectKey)
	if objectKey == "" {
		l.Errorf("产物对象路径不能为空")
		return nil, errorx.Msg("产物对象路径不能为空")
	}
	bucket := strings.TrimSpace(l.svcCtx.Config.StorageConf.BucketName)
	if bucket == "" {
		l.Errorf("产物存储桶未配置")
		return nil, errorx.Msg("产物存储桶未配置")
	}

	return &pb.PipelineArtifactUrlResp{Url: artifactPublicURL(l.svcCtx.Config.StorageConf.EndpointProxy, bucket, objectKey)}, nil
}
