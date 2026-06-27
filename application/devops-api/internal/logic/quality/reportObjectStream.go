package quality

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

type ReportObjectStream struct {
	Reader      io.ReadCloser
	Size        int64
	ContentType string
	FileName    string
}

func openScanReportObject(ctx context.Context, svcCtx *svc.ServiceContext, reqID string, preview bool) (*ReportObjectStream, error) {
	if svcCtx.Storage == nil {
		return nil, errorx.Msg("对象存储未配置")
	}
	projectIDs, restricted, err := resolveProjectScope(ctx, svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := svcCtx.QualityRpc.ScanReportGet(ctx, &qualityservice.GetScanReportReq{
		Id:                reqID,
		CurrentUserId:     currentUserID(ctx),
		CurrentRoles:      currentRoles(ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	report := result.Data
	if report == nil || strings.TrimSpace(report.ObjectKey) == "" {
		return nil, errorx.Msg("扫描报告对象地址不存在")
	}
	if preview && report.Size > maxReportPreviewBytes {
		return nil, errorx.Msg("报告过大，请下载查看")
	}
	bucket := firstNonEmptyAPI(report.Bucket, svcCtx.Config.StorageConf.BucketName)
	reader, size, contentType, err := svcCtx.Storage.Download(ctx, bucket, report.ObjectKey)
	if err != nil {
		return nil, err
	}
	if preview && size > maxReportPreviewBytes {
		_ = reader.Close()
		return nil, errorx.Msg("报告过大，请下载查看")
	}
	fileName := firstNonEmptyAPI(report.OriginalFileName, report.ArchiveEntryPath, report.ReportPath, report.Id)
	return &ReportObjectStream{
		Reader:      reader,
		Size:        size,
		ContentType: firstNonEmptyAPI(report.ContentType, contentType, detectReportContentType(fileName, nil)),
		FileName:    safeDownloadFileName(fileName),
	}, nil
}

func openUploadBatchObject(ctx context.Context, svcCtx *svc.ServiceContext, reqID string) (*ReportObjectStream, error) {
	if svcCtx.Storage == nil {
		return nil, errorx.Msg("对象存储未配置")
	}
	projectIDs, restricted, err := resolveProjectScope(ctx, svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := svcCtx.QualityRpc.ReportUploadBatchGet(ctx, &qualityservice.GetReportUploadBatchReq{
		Id:                reqID,
		CurrentUserId:     currentUserID(ctx),
		CurrentRoles:      currentRoles(ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	batch := result.Data
	if batch == nil || strings.TrimSpace(batch.ObjectKey) == "" {
		return nil, errorx.Msg("上传批次原件不存在")
	}
	bucket := firstNonEmptyAPI(batch.Bucket, svcCtx.Config.StorageConf.BucketName)
	reader, size, contentType, err := svcCtx.Storage.Download(ctx, bucket, batch.ObjectKey)
	if err != nil {
		return nil, err
	}
	fileName := firstNonEmptyAPI(batch.OriginalFileName, batch.Id)
	return &ReportObjectStream{
		Reader:      reader,
		Size:        size,
		ContentType: firstNonEmptyAPI(batch.ContentType, contentType, detectReportContentType(fileName, nil)),
		FileName:    safeDownloadFileName(fileName),
	}, nil
}

func openUploadEntryObject(ctx context.Context, svcCtx *svc.ServiceContext, reqID string, preview bool) (*ReportObjectStream, error) {
	if svcCtx.Storage == nil {
		return nil, errorx.Msg("对象存储未配置")
	}
	projectIDs, restricted, err := resolveProjectScope(ctx, svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := svcCtx.QualityRpc.ReportUploadEntryGet(ctx, &qualityservice.GetReportUploadEntryReq{
		Id:                reqID,
		CurrentUserId:     currentUserID(ctx),
		CurrentRoles:      currentRoles(ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	entry := result.Data
	if entry == nil || strings.TrimSpace(entry.ObjectKey) == "" {
		return nil, errorx.Msg("上传明细对象不存在")
	}
	if preview && entry.Size > maxReportPreviewBytes {
		return nil, errorx.Msg("报告过大，请下载查看")
	}
	bucket := firstNonEmptyAPI(entry.Bucket, svcCtx.Config.StorageConf.BucketName)
	reader, size, contentType, err := svcCtx.Storage.Download(ctx, bucket, entry.ObjectKey)
	if err != nil {
		return nil, err
	}
	if preview && size > maxReportPreviewBytes {
		_ = reader.Close()
		return nil, errorx.Msg("报告过大，请下载查看")
	}
	fileName := firstNonEmptyAPI(entry.OriginalFileName, entry.EntryPath, entry.Id)
	return &ReportObjectStream{
		Reader:      reader,
		Size:        size,
		ContentType: firstNonEmptyAPI(entry.ContentType, contentType, detectReportContentType(fileName, nil)),
		FileName:    safeDownloadFileName(fileName),
	}, nil
}

func safeDownloadFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return "report"
	}
	return name
}
