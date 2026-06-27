// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityScanReportCompleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 完成扫描报告上报
func NewQualityScanReportCompleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanReportCompleteLogic {
	return &QualityScanReportCompleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanReportCompleteLogic) QualityScanReportComplete(req *types.CompleteQualityScanReportRequest) (resp *types.IdResponse, err error) {
	if l.svcCtx.Storage == nil {
		return nil, errorx.Msg("对象存储未配置")
	}
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	bucket := firstNonEmptyAPI(req.Bucket, l.svcCtx.Config.StorageConf.BucketName)
	reader, size, contentType, err := l.svcCtx.Storage.Download(l.ctx, bucket, req.ObjectKey)
	if err != nil {
		l.Errorf("读取扫描报告对象失败: %v", err)
		return nil, errorx.Msg("读取扫描报告失败")
	}
	defer reader.Close()
	data, err := readLimitedReportUpload(reader)
	if err != nil {
		return nil, errorx.Msg(err.Error())
	}
	actualSHA := fmt.Sprintf("%x", sha256.Sum256(data))
	if strings.TrimSpace(req.Sha256) != "" && !strings.EqualFold(strings.TrimSpace(req.Sha256), actualSHA) {
		return nil, errorx.Msg("报告 sha256 不匹配")
	}
	summaryJSON, issuesJSON, err := parseScanReport(req.Tool, req.Parser, req.ReportFormat, data)
	if err != nil {
		return nil, errorx.Msg(err.Error())
	}
	actualSize := int64(len(data))
	if actualSize == 0 {
		actualSize = size
	}
	req.Size = actualSize
	if strings.TrimSpace(req.ContentType) == "" {
		req.ContentType = firstNonEmptyAPI(contentType, detectReportContentType(req.ObjectKey, data))
	}
	req.Bucket = bucket
	req.Sha256 = actualSHA
	req.SummaryJson = summaryJSON
	req.IssuesJson = issuesJSON

	result, err := l.svcCtx.QualityRpc.ScanReportComplete(l.ctx, &qualityservice.CompleteScanReportReq{
		RunId:             req.RunId,
		StageId:           req.StageId,
		StepId:            req.StepId,
		Tool:              req.Tool,
		ReportFormat:      req.ReportFormat,
		TargetType:        req.TargetType,
		TargetName:        req.TargetName,
		ImageRef:          req.ImageRef,
		Bucket:            req.Bucket,
		ObjectKey:         req.ObjectKey,
		Size:              req.Size,
		ContentType:       req.ContentType,
		Sha256:            req.Sha256,
		MetadataJson:      req.MetadataJson,
		SummaryJson:       req.SummaryJson,
		IssuesJson:        req.IssuesJson,
		ReportPath:        req.ReportPath,
		Parser:            req.Parser,
		SourceType:        "ci_complete",
		Operator:          currentUsername(l.ctx),
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
