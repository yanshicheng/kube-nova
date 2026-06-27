package pipelineconfigservicelogic

import (
	"bytes"
	"context"
	"path"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadPipelineArtifactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadPipelineArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadPipelineArtifactLogic {
	return &UploadPipelineArtifactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UploadPipelineArtifactLogic) UploadPipelineArtifact(in *pb.UploadPipelineArtifactReq) (*pb.UploadPipelineArtifactResp, error) {
	if l.svcCtx.Storage == nil {
		l.Errorf("产物存储未配置")
		return nil, errorx.Msg("产物存储未配置")
	}
	objectKey := cleanArtifactObjectKey(in.ObjectKey)
	if objectKey == "" {
		l.Errorf("产物对象路径不能为空")
		return nil, errorx.Msg("产物对象路径不能为空")
	}
	if len(in.Data) == 0 {
		l.Errorf("产物内容不能为空")
		return nil, errorx.Msg("产物内容不能为空")
	}
	bucket := strings.TrimSpace(l.svcCtx.Config.StorageConf.BucketName)
	if bucket == "" {
		l.Errorf("产物存储桶未配置")
		return nil, errorx.Msg("产物存储桶未配置")
	}
	contentType := strings.TrimSpace(in.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := l.svcCtx.Storage.Upload(bucket, objectKey, bytes.NewReader(in.Data), int64(len(in.Data)), contentType); err != nil {
		l.Errorf("上传流水线产物失败: %v", err)
		return nil, err
	}

	return &pb.UploadPipelineArtifactResp{
		Bucket:    bucket,
		ObjectKey: objectKey,
		Url:       artifactPublicURL(l.svcCtx.Config.StorageConf.EndpointProxy, bucket, objectKey),
		Size:      int64(len(in.Data)),
	}, nil
}

func cleanArtifactObjectKey(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	return path.Clean(value)
}

func artifactPublicURL(endpoint, bucket, objectKey string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" || bucket == "" || objectKey == "" {
		return ""
	}
	return endpoint + "/" + strings.Trim(bucket, "/") + "/" + strings.Trim(objectKey, "/")
}
