package logservicelogic

import (
	"database/sql"
	"encoding/json"
	"strings"
)

type logAlertEventPayload struct {
	RuleId          uint64          `json:"ruleId,omitempty"`
	RuleName        string          `json:"ruleName,omitempty"`
	RuleDescription string          `json:"ruleDescription,omitempty"`
	BackendType     string          `json:"backendType,omitempty"`
	ClusterUuid     string          `json:"clusterUuid,omitempty"`
	ClusterName     string          `json:"clusterName,omitempty"`
	ProjectUuid     string          `json:"projectUuid,omitempty"`
	ProjectName     string          `json:"projectName,omitempty"`
	WorkspaceId     uint64          `json:"workspaceId,omitempty"`
	WorkspaceName   string          `json:"workspaceName,omitempty"`
	Namespace       string          `json:"namespace,omitempty"`
	Application     string          `json:"application,omitempty"`
	ResourceName    string          `json:"resourceName,omitempty"`
	Status          string          `json:"status,omitempty"`
	HitCount        int64           `json:"hitCount,omitempty"`
	WindowStart     int64           `json:"windowStart,omitempty"`
	WindowEnd       int64           `json:"windowEnd,omitempty"`
	Sample          *logAlertSample `json:"sample,omitempty"`
	CloseReason     string          `json:"closeReason,omitempty"`
	ClosedBy        string          `json:"closedBy,omitempty"`
	ClosedAt        int64           `json:"closedAt,omitempty"`
	ManualReason    string          `json:"manualReason,omitempty"`
	ManualReasonBy  string          `json:"manualReasonBy,omitempty"`
	ManualReasonAt  int64           `json:"manualReasonAt,omitempty"`
}

type logAlertSample struct {
	Timestamp int64             `json:"timestamp,omitempty"`
	Message   string            `json:"message,omitempty"`
	Raw       string            `json:"raw,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func parseLogAlertEventPayload(payload sql.NullString) (*logAlertEventPayload, error) {
	if !payload.Valid || strings.TrimSpace(payload.String) == "" {
		return &logAlertEventPayload{}, nil
	}
	var result logAlertEventPayload
	if err := json.Unmarshal([]byte(payload.String), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func toEventPayloadJSON(payload *logAlertEventPayload) sql.NullString {
	if payload == nil {
		return sql.NullString{}
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return sql.NullString{}
	}
	return sql.NullString{String: string(data), Valid: true}
}
