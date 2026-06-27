package jenkins

import (
	"github.com/yanshicheng/kube-nova/common/devops/internal/httpcheck"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
)

func New() devopstypes.Provider {
	return httpcheck.ServiceProvider{
		ProductName: "Jenkins",
		Probes: []httpcheck.Probe{
			{Path: "/api/json", Capability: "remoteApi"},
			{Path: "/", Capability: "web"},
		},
	}
}
