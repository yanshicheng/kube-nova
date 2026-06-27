// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityScanReportUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	r      *http.Request
}

// 上传扫描报告
func NewQualityScanReportUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request) *QualityScanReportUploadLogic {
	return &QualityScanReportUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		r:      r,
	}
}

func (l *QualityScanReportUploadLogic) QualityScanReportUpload(req *types.UploadQualityScanReportRequest) (resp *types.QualityReportUploadResponse, err error) {
	if l.svcCtx.Storage == nil {
		return nil, errorx.Msg("对象存储未配置")
	}
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	if err := l.r.ParseMultipartForm(32 << 20); err != nil {
		l.Errorf("解析扫描报告上传表单失败: %v", err)
		return nil, errorx.Msg("解析上传表单失败")
	}
	if l.r.MultipartForm != nil {
		defer func(form *multipart.Form) {
			_ = form.RemoveAll()
		}(l.r.MultipartForm)
	}
	file, header, err := l.r.FormFile("file")
	if err != nil {
		file, header, err = firstUploadedFile(l.r)
	}
	if err != nil {
		return nil, errorx.Msg("请选择扫描报告文件")
	}
	defer file.Close()

	rawData, err := readLimitedReportUpload(file)
	if err != nil {
		return nil, errorx.Msg(err.Error())
	}
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectReportContentType(header.Filename, rawData)
	}
	bucket := l.svcCtx.Config.StorageConf.BucketName
	rawObjectKey := makeReportBatchObjectKey(req.RunId, req.Tool, header.Filename)
	rawSHA := fmt.Sprintf("%x", sha256.Sum256(rawData))

	operator := currentUsername(l.ctx)
	userID := currentUserID(l.ctx)
	roles := currentRoles(l.ctx)
	batch, err := l.svcCtx.QualityRpc.ReportUploadBatchCreate(l.ctx, &qualityservice.CreateReportUploadBatchReq{
		RunId:             req.RunId,
		StageId:           req.StageId,
		StepId:            req.StepId,
		Tool:              req.Tool,
		ReportFormat:      req.ReportFormat,
		Parser:            req.Parser,
		SourceType:        reportSourceTypeCIUpload,
		OriginalFileName:  header.Filename,
		Bucket:            bucket,
		ObjectKey:         rawObjectKey,
		Size:              int64(len(rawData)),
		ContentType:       contentType,
		Sha256:            rawSHA,
		Operator:          operator,
		CurrentUserId:     userID,
		CurrentRoles:      roles,
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Storage.Upload(bucket, rawObjectKey, bytes.NewReader(rawData), int64(len(rawData)), contentType); err != nil {
		l.Errorf("上传扫描报告批次原件到对象存储失败: %v", err)
		_, _ = l.svcCtx.QualityRpc.ReportUploadBatchUpdate(l.ctx, &qualityservice.UpdateReportUploadBatchReq{
			Id:                batch.Id,
			Status:            "failed",
			Message:           "上传扫描报告失败",
			Operator:          operator,
			CurrentUserId:     userID,
			CurrentRoles:      roles,
			ProjectIds:        projectIDs,
			ProjectRestricted: restricted,
		})
		return nil, errorx.Msg("上传扫描报告失败")
	}
	resp = &types.QualityReportUploadResponse{
		BatchId: batch.Id,
		Entries: make([]types.QualityReportUploadEntryResult, 0),
	}

	entries, err := expandReportUpload(header.Filename, req.ReportPath, rawData)
	if err != nil {
		entries = []reportArchiveEntry{rejectedArchiveEntry(header.Filename, err.Error())}
	}
	for _, entry := range entries {
		result, err := l.handleReportUploadEntry(req, batch.Id, bucket, operator, projectIDs, restricted, entry)
		if err != nil {
			return nil, err
		}
		resp.Entries = append(resp.Entries, result)
		switch result.Status {
		case "accepted":
			resp.AcceptedCount++
			if result.ReportId != "" {
				resp.ReportIds = append(resp.ReportIds, result.ReportId)
			}
		case "ignored":
			resp.IgnoredCount++
		default:
			resp.RejectedCount++
		}
	}
	status := "success"
	message := "扫描报告上传完成"
	if resp.AcceptedCount == 0 && (resp.RejectedCount > 0 || resp.IgnoredCount > 0) {
		status = "failed"
		message = "扫描报告上传失败"
	} else if resp.RejectedCount > 0 || resp.IgnoredCount > 0 {
		status = "partial"
		message = "扫描报告部分处理成功"
	}
	if _, err := l.svcCtx.QualityRpc.ReportUploadBatchUpdate(l.ctx, &qualityservice.UpdateReportUploadBatchReq{
		Id:                batch.Id,
		Status:            status,
		AcceptedCount:     resp.AcceptedCount,
		RejectedCount:     resp.RejectedCount,
		IgnoredCount:      resp.IgnoredCount,
		Message:           message,
		Operator:          operator,
		CurrentUserId:     userID,
		CurrentRoles:      roles,
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	}); err != nil {
		l.Errorf("更新扫描报告上传批次状态失败: %v", err)
		return nil, err
	}
	return resp, nil
}

func firstUploadedFile(r *http.Request) (multipart.File, *multipart.FileHeader, error) {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, nil, http.ErrMissingFile
	}
	for _, files := range r.MultipartForm.File {
		if len(files) == 0 {
			continue
		}
		file, err := files[0].Open()
		return file, files[0], err
	}
	return nil, nil, http.ErrMissingFile
}

func (l *QualityScanReportUploadLogic) handleReportUploadEntry(req *types.UploadQualityScanReportRequest, batchID, bucket, operator string, projectIDs []string, restricted bool, entry reportArchiveEntry) (types.QualityReportUploadEntryResult, error) {
	result := types.QualityReportUploadEntryResult{
		EntryPath: entry.EntryPath,
		Status:    entry.Status,
		Reason:    entry.Reason,
		Size:      entry.Size,
	}
	if result.Status != "" {
		id, err := l.createReportUploadEntry(req, batchID, bucket, entry, result.Status, result.Reason, "")
		result.Id = id
		return result, err
	}
	entrySHA := fmt.Sprintf("%x", sha256.Sum256(entry.Data))
	objectKey := makeReportObjectKey(req.RunId, req.Tool, entry.EntryPath)
	if err := l.svcCtx.Storage.Upload(bucket, objectKey, bytes.NewReader(entry.Data), int64(len(entry.Data)), entry.ContentType); err != nil {
		l.Errorf("上传解压后的扫描报告失败: %v", err)
		result.Status = "rejected"
		result.Reason = "上传解压后的扫描报告失败"
		id, createErr := l.createReportUploadEntry(req, batchID, bucket, entry, result.Status, result.Reason, "")
		result.Id = id
		return result, createErr
	}
	entry.Sha256 = entrySHA
	result.ObjectKey = objectKey
	result.Sha256 = entrySHA
	result.Size = int64(len(entry.Data))
	summaryJSON, issuesJSON, err := parseScanReport(req.Tool, req.Parser, req.ReportFormat, entry.Data)
	if err != nil {
		result.Status = "rejected"
		result.Reason = err.Error()
		id, createErr := l.createReportUploadEntryWithObject(req, batchID, bucket, objectKey, entry, result.Status, result.Reason, "")
		result.Id = id
		return result, createErr
	}
	completeReq := &qualityservice.CompleteScanReportReq{
		RunId:             req.RunId,
		StageId:           req.StageId,
		StepId:            req.StepId,
		Tool:              req.Tool,
		ReportFormat:      req.ReportFormat,
		TargetType:        req.TargetType,
		TargetName:        req.TargetName,
		ImageRef:          req.ImageRef,
		Bucket:            bucket,
		ObjectKey:         objectKey,
		Size:              int64(len(entry.Data)),
		ContentType:       entry.ContentType,
		Sha256:            entrySHA,
		MetadataJson:      req.MetadataJson,
		SummaryJson:       summaryJSON,
		IssuesJson:        issuesJSON,
		ReportPath:        entry.EntryPath,
		Parser:            req.Parser,
		BatchId:           batchID,
		SourceType:        reportSourceTypeCIUpload,
		ArchiveEntryPath:  entry.EntryPath,
		OriginalFileName:  entry.OriginalFileName,
		Operator:          operator,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	}
	if _, err := l.svcCtx.QualityRpc.ScanReportValidate(l.ctx, completeReq); err != nil {
		result.Status = "rejected"
		result.Reason = err.Error()
		id, createErr := l.createReportUploadEntryWithObject(req, batchID, bucket, objectKey, entry, result.Status, result.Reason, "")
		result.Id = id
		return result, createErr
	}
	completed, err := l.svcCtx.QualityRpc.ScanReportComplete(l.ctx, completeReq)
	if err != nil {
		result.Status = "rejected"
		result.Reason = err.Error()
		id, createErr := l.createReportUploadEntryWithObject(req, batchID, bucket, objectKey, entry, result.Status, result.Reason, "")
		result.Id = id
		return result, createErr
	}
	result.Status = "accepted"
	result.ReportId = completed.Id
	id, err := l.createReportUploadEntryWithObject(req, batchID, bucket, objectKey, entry, result.Status, "", completed.Id)
	result.Id = id
	return result, err
}

func (l *QualityScanReportUploadLogic) createReportUploadEntry(req *types.UploadQualityScanReportRequest, batchID, bucket string, entry reportArchiveEntry, status, reason, reportID string) (string, error) {
	return l.createReportUploadEntryWithObject(req, batchID, bucket, "", entry, status, reason, reportID)
}

func (l *QualityScanReportUploadLogic) createReportUploadEntryWithObject(req *types.UploadQualityScanReportRequest, batchID, bucket, objectKey string, entry reportArchiveEntry, status, reason, reportID string) (string, error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return "", err
	}
	created, err := l.svcCtx.QualityRpc.ReportUploadEntryCreate(l.ctx, &qualityservice.CreateReportUploadEntryReq{
		BatchId:           batchID,
		RunId:             req.RunId,
		StageId:           req.StageId,
		StepId:            req.StepId,
		Tool:              req.Tool,
		ReportFormat:      req.ReportFormat,
		Parser:            req.Parser,
		EntryPath:         entry.EntryPath,
		OriginalFileName:  entry.OriginalFileName,
		Bucket:            bucket,
		ObjectKey:         objectKey,
		Size:              entry.Size,
		ContentType:       entry.ContentType,
		Sha256:            entry.Sha256,
		Status:            status,
		Reason:            reason,
		ReportId:          reportID,
		Operator:          currentUsername(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return "", err
	}
	return created.Id, nil
}
