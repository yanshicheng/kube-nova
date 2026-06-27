package quality

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

var objectKeyUnsafePattern = regexp.MustCompile(`[^a-zA-Z0-9._/-]+`)

func currentUsername(ctx context.Context) string {
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return "system"
	}
	return username
}

func currentUserID(ctx context.Context) uint64 {
	value := ctx.Value("userId")
	switch v := value.(type) {
	case uint64:
		return v
	case uint:
		return uint64(v)
	case int:
		if v > 0 {
			return uint64(v)
		}
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case float64:
		if v > 0 {
			return uint64(v)
		}
	case string:
		id, _ := strconv.ParseUint(v, 10, 64)
		return id
	}
	return 0
}

func currentRoles(ctx context.Context) []string {
	value := ctx.Value("roles")
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		roles := make([]string, 0, len(v))
		for _, item := range v {
			if role, ok := item.(string); ok && role != "" {
				roles = append(roles, role)
			}
		}
		return roles
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func isSuperAdminRole(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

func resolveProjectScope(ctx context.Context, svcCtx *svc.ServiceContext, projectID string) ([]string, bool, error) {
	projectID = strings.TrimSpace(projectID)
	roles := currentRoles(ctx)
	if isSuperAdminRole(roles) {
		return nil, false, nil
	}
	ids, err := listAccessibleProjectIDs(ctx, svcCtx.ProjectRpc, currentUserID(ctx), roles)
	if err != nil {
		return nil, true, err
	}
	if projectID == "" {
		return ids, true, nil
	}
	for _, id := range ids {
		if id == projectID {
			return []string{projectID}, true, nil
		}
	}
	return nil, true, errorx.Msg("无权访问当前 DevOps 项目")
}

func listAccessibleProjectIDs(ctx context.Context, projectRpc projectservice.ProjectService, userID uint64, roles []string) ([]string, error) {
	const pageSize uint64 = 200
	page := uint64(1)
	ids := make([]string, 0)
	for {
		resp, err := projectRpc.ProjectList(ctx, &projectservice.ListProjectReq{
			Page:          page,
			PageSize:      pageSize,
			Status:        1,
			CurrentUserId: userID,
			CurrentRoles:  roles,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			if item != nil && item.Id != "" {
				ids = append(ids, item.Id)
			}
		}
		if uint64(len(ids)) >= resp.Total || len(resp.Data) == 0 {
			return ids, nil
		}
		page++
	}
}

func scanPlanItemToRpc(in types.QualityScanPlanItem) *qualityservice.ScanPlanItem {
	return &qualityservice.ScanPlanItem{
		StepId:          in.StepId,
		StageId:         in.StageId,
		Tool:            in.Tool,
		ToolMode:        in.ToolMode,
		ToolBindingId:   in.ToolBindingId,
		TargetType:      in.TargetType,
		TargetName:      in.TargetName,
		TargetParamCode: in.TargetParamCode,
		ReportSource:    in.ReportSource,
		ReportPath:      in.ReportPath,
		ReportFormat:    in.ReportFormat,
		Required:        in.Required,
		Enforce:         in.Enforce,
		Parser:          in.Parser,
	}
}

func scanPlanItemToType(in *qualityservice.ScanPlanItem) types.QualityScanPlanItem {
	if in == nil {
		return types.QualityScanPlanItem{}
	}
	return types.QualityScanPlanItem{
		StepId:          in.StepId,
		StageId:         in.StageId,
		Tool:            in.Tool,
		ToolMode:        in.ToolMode,
		ToolBindingId:   in.ToolBindingId,
		TargetType:      in.TargetType,
		TargetName:      in.TargetName,
		TargetParamCode: in.TargetParamCode,
		ReportSource:    in.ReportSource,
		ReportPath:      in.ReportPath,
		ReportFormat:    in.ReportFormat,
		Required:        in.Required,
		Enforce:         in.Enforce,
		Parser:          in.Parser,
	}
}

func scanReportToType(in *qualityservice.ScanReport) types.QualityScanReport {
	if in == nil {
		return types.QualityScanReport{}
	}
	return types.QualityScanReport{
		Id:               in.Id,
		ProjectId:        in.ProjectId,
		ProjectName:      in.ProjectName,
		ProjectCode:      in.ProjectCode,
		SystemId:         in.SystemId,
		SystemName:       in.SystemName,
		SystemCode:       in.SystemCode,
		EnvironmentId:    in.EnvironmentId,
		EnvironmentName:  in.EnvironmentName,
		EnvironmentCode:  in.EnvironmentCode,
		PipelineId:       in.PipelineId,
		RunId:            in.RunId,
		StageId:          in.StageId,
		StepId:           in.StepId,
		Tool:             in.Tool,
		ToolMode:         in.ToolMode,
		ToolBindingId:    in.ToolBindingId,
		TargetType:       in.TargetType,
		TargetName:       in.TargetName,
		ImageRef:         in.ImageRef,
		ReportSource:     in.ReportSource,
		ReportFormat:     in.ReportFormat,
		ReportPath:       in.ReportPath,
		Parser:           in.Parser,
		Bucket:           in.Bucket,
		ObjectKey:        in.ObjectKey,
		Size:             in.Size,
		ContentType:      in.ContentType,
		Sha256:           in.Sha256,
		Status:           in.Status,
		RejectReason:     in.RejectReason,
		GateStatus:       in.GateStatus,
		SummaryJson:      in.SummaryJson,
		BatchId:          in.BatchId,
		EntryId:          in.EntryId,
		SourceType:       in.SourceType,
		ArchiveEntryPath: in.ArchiveEntryPath,
		OriginalFileName: in.OriginalFileName,
		Current:          in.Current,
		SupersededBy:     in.SupersededBy,
		CreatedAt:        in.CreatedAt,
	}
}

func scanIssueToType(in *qualityservice.ScanIssue) types.QualityScanIssue {
	if in == nil {
		return types.QualityScanIssue{}
	}
	return types.QualityScanIssue{
		Id:               in.Id,
		ReportId:         in.ReportId,
		ProjectId:        in.ProjectId,
		SystemId:         in.SystemId,
		RunId:            in.RunId,
		Tool:             in.Tool,
		TargetType:       in.TargetType,
		IssueType:        in.IssueType,
		Severity:         in.Severity,
		Title:            in.Title,
		Description:      in.Description,
		FilePath:         in.FilePath,
		Line:             in.Line,
		Component:        in.Component,
		PackageName:      in.PackageName,
		ImageRef:         in.ImageRef,
		ImageDigest:      in.ImageDigest,
		CveId:            in.CveId,
		InstalledVersion: in.InstalledVersion,
		FixedVersion:     in.FixedVersion,
		RuleId:           in.RuleId,
		Status:           in.Status,
		RawJson:          in.RawJson,
		CreatedAt:        in.CreatedAt,
	}
}

func dashboardOverviewToType(in *qualityservice.QualityDashboardOverview) types.QualityDashboardOverview {
	if in == nil {
		return types.QualityDashboardOverview{}
	}
	return types.QualityDashboardOverview{
		ReportCount:   in.ReportCount,
		PassedCount:   in.PassedCount,
		FailedCount:   in.FailedCount,
		RejectedCount: in.RejectedCount,
		IssueCount:    in.IssueCount,
		CriticalCount: in.CriticalCount,
		HighCount:     in.HighCount,
		GatePassRate:  in.GatePassRate,
	}
}

func runTypeSummaryToType(in *qualityservice.QualityRunTypeSummary) types.QualityRunTypeSummary {
	if in == nil {
		return types.QualityRunTypeSummary{}
	}
	return types.QualityRunTypeSummary{
		TargetType:    in.TargetType,
		ReportCount:   in.ReportCount,
		IssueCount:    in.IssueCount,
		CriticalCount: in.CriticalCount,
		HighCount:     in.HighCount,
		MediumCount:   in.MediumCount,
		LowCount:      in.LowCount,
	}
}

func imageSummariesToType(items []*qualityservice.QualityImageSummary) []types.QualityImageSummary {
	result := make([]types.QualityImageSummary, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.QualityImageSummary{
			ImageRef:      item.ImageRef,
			ImageDigest:   item.ImageDigest,
			ReportCount:   item.ReportCount,
			IssueCount:    item.IssueCount,
			CriticalCount: item.CriticalCount,
			HighCount:     item.HighCount,
			FixableCount:  item.FixableCount,
		})
	}
	return result
}

func scanReportsToType(items []*qualityservice.ScanReport) []types.QualityScanReport {
	result := make([]types.QualityScanReport, 0, len(items))
	for _, item := range items {
		result = append(result, scanReportToType(item))
	}
	return result
}

func scanIssuesToType(items []*qualityservice.ScanIssue) []types.QualityScanIssue {
	result := make([]types.QualityScanIssue, 0, len(items))
	for _, item := range items {
		result = append(result, scanIssueToType(item))
	}
	return result
}

func missingReportsToType(items []*qualityservice.ScanGateMissingReport) []types.QualityGateMissingReport {
	result := make([]types.QualityGateMissingReport, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.QualityGateMissingReport{
			Tool:         item.Tool,
			StepId:       item.StepId,
			ReportFormat: item.ReportFormat,
			ReportPath:   item.ReportPath,
		})
	}
	return result
}

func scanPlanItemsToType(items []*qualityservice.ScanPlanItem) []types.QualityScanPlanItem {
	result := make([]types.QualityScanPlanItem, 0, len(items))
	for _, item := range items {
		result = append(result, scanPlanItemToType(item))
	}
	return result
}

func uploadBatchToType(in *qualityservice.ReportUploadBatch) types.QualityReportUploadBatch {
	if in == nil {
		return types.QualityReportUploadBatch{}
	}
	return types.QualityReportUploadBatch{
		Id:               in.Id,
		ProjectId:        in.ProjectId,
		ProjectName:      in.ProjectName,
		ProjectCode:      in.ProjectCode,
		SystemId:         in.SystemId,
		SystemName:       in.SystemName,
		SystemCode:       in.SystemCode,
		EnvironmentId:    in.EnvironmentId,
		EnvironmentName:  in.EnvironmentName,
		EnvironmentCode:  in.EnvironmentCode,
		PipelineId:       in.PipelineId,
		PipelineName:     in.PipelineName,
		RunId:            in.RunId,
		StageId:          in.StageId,
		StepId:           in.StepId,
		Tool:             in.Tool,
		ReportFormat:     in.ReportFormat,
		Parser:           in.Parser,
		SourceType:       in.SourceType,
		OriginalFileName: in.OriginalFileName,
		Bucket:           in.Bucket,
		ObjectKey:        in.ObjectKey,
		Size:             in.Size,
		ContentType:      in.ContentType,
		Sha256:           in.Sha256,
		Status:           in.Status,
		AcceptedCount:    in.AcceptedCount,
		RejectedCount:    in.RejectedCount,
		IgnoredCount:     in.IgnoredCount,
		Message:          in.Message,
		CreatedAt:        in.CreatedAt,
	}
}

func uploadEntryToType(in *qualityservice.ReportUploadEntry) types.QualityReportUploadEntry {
	if in == nil {
		return types.QualityReportUploadEntry{}
	}
	return types.QualityReportUploadEntry{
		Id:               in.Id,
		BatchId:          in.BatchId,
		RunId:            in.RunId,
		StageId:          in.StageId,
		StepId:           in.StepId,
		Tool:             in.Tool,
		ReportFormat:     in.ReportFormat,
		Parser:           in.Parser,
		EntryPath:        in.EntryPath,
		OriginalFileName: in.OriginalFileName,
		Bucket:           in.Bucket,
		ObjectKey:        in.ObjectKey,
		Size:             in.Size,
		ContentType:      in.ContentType,
		Sha256:           in.Sha256,
		Status:           in.Status,
		Reason:           in.Reason,
		ReportId:         in.ReportId,
		CreatedAt:        in.CreatedAt,
	}
}

func uploadBatchesToType(items []*qualityservice.ReportUploadBatch) []types.QualityReportUploadBatch {
	result := make([]types.QualityReportUploadBatch, 0, len(items))
	for _, item := range items {
		result = append(result, uploadBatchToType(item))
	}
	return result
}

func uploadEntriesToType(items []*qualityservice.ReportUploadEntry) []types.QualityReportUploadEntry {
	result := make([]types.QualityReportUploadEntry, 0, len(items))
	for _, item := range items {
		result = append(result, uploadEntryToType(item))
	}
	return result
}

func objectPublicURL(endpoint, bucket, objectKey string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	bucket = strings.Trim(strings.TrimSpace(bucket), "/")
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	if endpoint == "" || bucket == "" || objectKey == "" {
		return ""
	}
	return endpoint + "/" + bucket + "/" + objectKey
}

func makeReportObjectKey(runID, tool, fileName string) string {
	name := filepath.Base(strings.TrimSpace(fileName))
	if name == "." || name == "/" || name == "" {
		name = "report"
	}
	safeRunID := objectKeyUnsafePattern.ReplaceAllString(strings.TrimSpace(runID), "_")
	if safeRunID == "" {
		safeRunID = "unknown"
	}
	safeName := objectKeyUnsafePattern.ReplaceAllString(name, "_")
	safeTool := objectKeyUnsafePattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(tool)), "_")
	if safeTool == "" {
		safeTool = "unknown"
	}
	ts := time.Now().Format("20060102150405")
	return fmt.Sprintf("quality-reports/%s/%s/%s-%s", safeRunID, safeTool, ts, safeName)
}

func makeReportBatchObjectKey(runID, tool, fileName string) string {
	name := filepath.Base(strings.TrimSpace(fileName))
	if name == "." || name == "/" || name == "" {
		name = "upload"
	}
	safeRunID := objectKeyUnsafePattern.ReplaceAllString(strings.TrimSpace(runID), "_")
	if safeRunID == "" {
		safeRunID = "unknown"
	}
	safeName := objectKeyUnsafePattern.ReplaceAllString(name, "_")
	safeTool := objectKeyUnsafePattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(tool)), "_")
	if safeTool == "" {
		safeTool = "unknown"
	}
	ts := time.Now().Format("20060102150405")
	return fmt.Sprintf("quality-report-batches/%s/%s/%s-%s", safeRunID, safeTool, ts, safeName)
}
