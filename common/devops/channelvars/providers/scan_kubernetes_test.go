package providers

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

func TestSonarQubeResolveAddressUsesStandardScanFields(t *testing.T) {
	provider := NewSonarQubeProvider(fakeImageChannelManager{channelType: "sonarqube"})
	resp, err := provider.ResolveAddress(context.Background(), &channelvars.ResolveAddressRequest{
		Address:   "https://sonar.example.com/dashboard?id=demo-service",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("resolve sonarqube address: %v", err)
	}
	if !resp.Resolved {
		t.Fatalf("sonarqube address should be resolved: %s", resp.Reason)
	}
	assertField(t, resp.Fields, channelvars.FieldDynamicProjectName, "demo-service")
	assertField(t, resp.Fields, channelvars.FieldDynamicProjectKey, "demo-service")
}

func TestKubernetesProviderSupportsStandardQueryKeys(t *testing.T) {
	provider := NewKubernetesProvider(fakeImageChannelManager{channelType: "kubernetes"}, fakeK8sManager{})

	resp, err := provider.QueryOptions(context.Background(), &channelvars.QueryOptionsRequest{
		ChannelInstanceID: 1,
		ProviderKey:       "kubernetes.namespace",
	})
	if err != nil {
		t.Fatalf("query namespaces: %v", err)
	}
	assertOption(t, resp.Options, "default")

	resp, err = provider.QueryOptions(context.Background(), &channelvars.QueryOptionsRequest{
		ChannelInstanceID: 1,
		ProviderKey:       "kubernetes.resourceName",
		ParentValues: map[string]string{
			channelvars.FieldDynamicNamespace:    "default",
			channelvars.FieldDynamicWorkloadType: "deployment",
		},
	})
	if err != nil {
		t.Fatalf("query resources: %v", err)
	}
	assertOption(t, resp.Options, "demo-app")

	resp, err = provider.QueryOptions(context.Background(), &channelvars.QueryOptionsRequest{
		ChannelInstanceID: 1,
		ProviderKey:       "kubernetes.containerName",
		ParentValues: map[string]string{
			channelvars.FieldDynamicNamespace:    "default",
			channelvars.FieldDynamicWorkloadType: "deployment",
			channelvars.FieldDynamicResourceName: "demo-app",
		},
	})
	if err != nil {
		t.Fatalf("query containers: %v", err)
	}
	assertOption(t, resp.Options, "app")

	resp, err = provider.QueryOptions(context.Background(), &channelvars.QueryOptionsRequest{
		ChannelInstanceID: 1,
		ProviderKey:       "kubernetes.imageName",
		ParentValues: map[string]string{
			channelvars.FieldDynamicNamespace:     "default",
			channelvars.FieldDynamicWorkloadType:  "deployment",
			channelvars.FieldDynamicResourceName:  "demo-app",
			channelvars.FieldDynamicContainerName: "app",
		},
	})
	if err != nil {
		t.Fatalf("query image: %v", err)
	}
	assertOption(t, resp.Options, "nginx:v1")
}

func TestKubernetesProviderKeepsLegacyQueryKeys(t *testing.T) {
	provider := NewKubernetesProvider(fakeImageChannelManager{channelType: "kubernetes"}, fakeK8sManager{})
	resp, err := provider.QueryOptions(context.Background(), &channelvars.QueryOptionsRequest{
		ChannelInstanceID: 1,
		ProviderKey:       "k8s.container",
		ParentValues: map[string]string{
			channelvars.FieldDynamicNamespace:    "default",
			channelvars.FieldDynamicWorkloadType: "deployment",
			channelvars.FieldDynamicResource:     "demo-app",
		},
	})
	if err != nil {
		t.Fatalf("query legacy containers: %v", err)
	}
	assertOption(t, resp.Options, "app")
}

func TestStandardProvidersSupportStandardVariableFields(t *testing.T) {
	providers := map[string]channelvars.Provider{
		"gitlab":     NewGitProvider(fakeImageChannelManager{channelType: "gitlab"}),
		"github":     NewGitProvider(fakeImageChannelManager{channelType: "github"}),
		"gitee":      NewGitProvider(fakeImageChannelManager{channelType: "gitee"}),
		"harbor":     NewHarborProvider(fakeImageChannelManager{channelType: "harbor"}),
		"registry":   NewRegistryProvider(fakeImageChannelManager{channelType: "registry"}),
		"nexus":      NewNexusProvider(fakeImageChannelManager{channelType: "nexus"}),
		"jfrog":      NewJFrogProvider(fakeImageChannelManager{channelType: "jfrog"}),
		"sonarqube":  NewSonarQubeProvider(fakeImageChannelManager{channelType: "sonarqube"}),
		"kubernetes": NewKubernetesProvider(fakeImageChannelManager{channelType: "kubernetes"}, fakeK8sManager{}),
		"host":       NewHostProvider(fakeImageChannelManager{channelType: "host"}),
		"host_group": NewHostGroupProvider(fakeImageChannelManager{channelType: "host_group"}, nil),
	}
	cases := []struct {
		groupCode   string
		channelType string
	}{
		{channelvars.GroupCodeRepo, "gitlab"},
		{channelvars.GroupCodeRepo, "github"},
		{channelvars.GroupCodeRepo, "gitee"},
		{channelvars.GroupImageRepo, "harbor"},
		{channelvars.GroupImageRepo, "registry"},
		{channelvars.GroupArtifactRepo, "nexus"},
		{channelvars.GroupArtifactRepo, "jfrog"},
		{channelvars.GroupCodeScan, "sonarqube"},
		{channelvars.GroupDeployTarget, "kubernetes"},
		{channelvars.GroupDeployTarget, "host"},
		{channelvars.GroupDeployTarget, "host_group"},
	}

	for _, tc := range cases {
		t.Run(tc.groupCode+"/"+tc.channelType, func(t *testing.T) {
			provider := providers[tc.channelType]
			if provider == nil {
				t.Fatalf("provider %s not found", tc.channelType)
			}
			capabilities := provider.Capabilities()
			querySet := make(map[string]bool, len(capabilities.SupportedQueries))
			for _, key := range capabilities.SupportedQueries {
				querySet[key] = true
			}
			addressSet := make(map[string]bool, len(capabilities.SupportedAddressFormats))
			for _, format := range capabilities.SupportedAddressFormats {
				addressSet[format.Field] = true
			}
			for _, field := range channelvars.StandardVariableFields(tc.groupCode, tc.channelType) {
				if field.Provider == "" || field.UIControl == "deployConfig" {
					continue
				}
				if channelvars.IsAddressVariableField(field) {
					if !querySet[field.Provider] && !addressSet[field.Field] {
						t.Fatalf("address field %s provider %s is not supported", field.Field, field.Provider)
					}
					continue
				}
				if !querySet[field.Provider] {
					t.Fatalf("field %s provider %s is not supported", field.Field, field.Provider)
				}
			}
		})
	}
}

func assertOption(t *testing.T, options []channelvars.Option, value string) {
	t.Helper()
	for _, option := range options {
		if option.Value == value {
			return
		}
	}
	t.Fatalf("option %q not found in %#v", value, options)
}

type fakeK8sManager struct{}

func (fakeK8sManager) ListNamespaces(context.Context, int64) ([]string, error) {
	return []string{"default"}, nil
}

func (fakeK8sManager) ListResources(context.Context, int64, string, string) ([]string, error) {
	return []string{"demo-app"}, nil
}

func (fakeK8sManager) ListContainers(context.Context, int64, string, string, string) ([]string, error) {
	return []string{"app"}, nil
}

func (fakeK8sManager) GetContainerImage(context.Context, int64, string, string, string, string) (string, error) {
	return "nginx:v1", nil
}
