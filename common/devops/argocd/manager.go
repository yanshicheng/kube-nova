package argocd

import (
	"github.com/yanshicheng/kube-nova/common/devops/internal/httpcheck"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
)

func New() devopstypes.Provider {
	return httpcheck.ServiceProvider{
		ProductName: "Argo CD",
		Probes: []httpcheck.Probe{
			{Path: "/api/version", Capability: "versionApi"},
			{Path: "/api/v1/session/userinfo", Capability: "sessionApi"},
		},
	}
}
