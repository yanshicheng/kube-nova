package project

import (
	"context"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
)

const containerPlatformId uint64 = 1

func currentPlatformID(ctx context.Context) uint64 {
	value := ctx.Value("platformId")
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

func isSuperAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

func projectToType(in *pb.PortalProject) types.Project {
	if in == nil {
		return types.Project{}
	}
	return types.Project{
		Id:          in.Id,
		Name:        in.Name,
		Uuid:        in.Uuid,
		Description: in.Description,
		IsSystem:    in.IsSystem,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}

func projectsToType(items []*pb.PortalProject) []types.Project {
	data := make([]types.Project, 0, len(items))
	for _, item := range items {
		data = append(data, projectToType(item))
	}
	return data
}
