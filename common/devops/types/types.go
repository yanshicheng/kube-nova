package types

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type Provider interface {
	TestConnection(ctx context.Context, req Request) Result
}

type Channel struct {
	ID              string
	Name            string
	Code            string
	Type            string
	Endpoint        string
	Config          string
	AuthType        string
	Username        string
	Password        string
	Token           string
	InsecureSkipTLS bool
}

type Credential struct {
	Type        string
	Username    string
	Password    string
	Token       string
	PrivateKey  string
	Passphrase  string
	Kubeconfig  string
	SecretText  string
	Certificate string
	JsonData    string
}

type Host struct {
	ID         string
	Name       string
	IP         string
	Port       int64
	Credential *Credential
}

type Request struct {
	Channel    Channel
	Credential *Credential
	Hosts      []Host
}

type Metadata struct {
	ProductName      string         `json:"productName,omitempty"`
	Version          string         `json:"version,omitempty"`
	ProductVersion   string         `json:"productVersion,omitempty"`
	EngineVersion    string         `json:"engineVersion,omitempty"`
	APIVersion       string         `json:"apiVersion,omitempty"`
	SchemaVersion    string         `json:"schemaVersion,omitempty"`
	BenchmarkProfile string         `json:"benchmarkProfile,omitempty"`
	AuthUser         string         `json:"authUser,omitempty"`
	Capabilities     map[string]any `json:"capabilities,omitempty"`
	HealthMessage    string         `json:"healthMessage,omitempty"`
	LastCheckedAt    int64          `json:"lastCheckedAt,omitempty"`
}

type Result struct {
	Success      bool
	HealthStatus string
	Message      string
	CheckedAt    int64
	Metadata     Metadata
}

type DynamicOption struct {
	Label    string
	Value    string
	Metadata map[string]any
}

func Healthy(message string, metadata Metadata) Result {
	checkedAt := time.Now().Unix()
	metadata = normalizeMetadata(metadata)
	metadata.LastCheckedAt = checkedAt
	metadata.HealthMessage = TrimMessage(message)
	return Result{
		Success:      true,
		HealthStatus: "healthy",
		Message:      TrimMessage(message),
		CheckedAt:    checkedAt,
		Metadata:     metadata,
	}
}

func Unhealthy(message string, metadata Metadata) Result {
	checkedAt := time.Now().Unix()
	metadata = normalizeMetadata(metadata)
	metadata.LastCheckedAt = checkedAt
	metadata.HealthMessage = TrimMessage(message)
	return Result{
		Success:      false,
		HealthStatus: "unhealthy",
		Message:      TrimMessage(message),
		CheckedAt:    checkedAt,
		Metadata:     metadata,
	}
}

func MetadataJSON(metadata Metadata) string {
	metadata = normalizeMetadata(metadata)
	data, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func normalizeMetadata(metadata Metadata) Metadata {
	if metadata.Version == "" {
		for _, value := range []string{
			metadata.ProductVersion,
			metadata.EngineVersion,
			metadata.SchemaVersion,
			metadata.BenchmarkProfile,
		} {
			if value = strings.TrimSpace(value); value != "" {
				metadata.Version = value
				break
			}
		}
	}
	if metadata.Version == "" && shouldUseAPIVersionAsVersion(metadata.ProductName) {
		metadata.Version = strings.TrimSpace(metadata.APIVersion)
	}
	return metadata
}

func shouldUseAPIVersionAsVersion(productName string) bool {
	switch strings.ToLower(strings.TrimSpace(productName)) {
	case "docker registry", "github", "gitee", "tekton":
		return true
	default:
		return false
	}
}

func TrimMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 240 {
		return message[:240]
	}
	return message
}
