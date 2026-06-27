package channelvars

import (
	"context"
	"testing"
)

func TestStandardVariableProvidersAreRegisteredCapabilities(t *testing.T) {
	providers := map[string]Provider{
		"gitlab":     fakeCapabilityProvider{queries: []string{"git.project", "git.branch", "git.tag"}, addresses: []string{"address.projectUrl"}},
		"github":     fakeCapabilityProvider{queries: []string{"git.project", "git.branch", "git.tag"}, addresses: []string{"address.projectUrl"}},
		"gitee":      fakeCapabilityProvider{queries: []string{"git.project", "git.branch", "git.tag"}, addresses: []string{"address.projectUrl"}},
		"svn":        fakeCapabilityProvider{queries: []string{"svn.path", "svn.revision"}},
		"harbor":     fakeCapabilityProvider{queries: []string{"image.project", "image.image", "image.tag", "image.imageTag"}, addresses: []string{"address.projectUrl", "address.imageUrl", "address.imageTagUrl"}},
		"registry":   fakeCapabilityProvider{queries: []string{"image.project", "image.image", "image.tag", "image.imageTag"}, addresses: []string{"address.projectUrl", "address.imageUrl", "address.imageTagUrl"}},
		"nexus":      fakeCapabilityProvider{queries: []string{"artifact.repository", "artifact.artifactName", "artifact.artifactVersion"}, addresses: []string{"address.repositoryUrl", "address.artifactUrl", "address.artifactVersionUrl"}},
		"jfrog":      fakeCapabilityProvider{queries: []string{"artifact.repository", "artifact.artifactName", "artifact.artifactVersion"}, addresses: []string{"address.repositoryUrl", "address.artifactUrl", "address.artifactVersionUrl"}},
		"sonarqube":  fakeCapabilityProvider{queries: []string{"scan.projectName", "scan.projectKey"}},
		"kubernetes": fakeCapabilityProvider{queries: []string{"kubernetes.namespace", "kubernetes.resourceName", "kubernetes.containerName", "kubernetes.imageName"}},
		"host":       fakeCapabilityProvider{addresses: []string{"address.hostConfig"}},
		"host_group": fakeCapabilityProvider{addresses: []string{"address.groupConfig"}},
	}
	cases := []struct {
		groupCode   string
		channelType string
	}{
		{GroupCodeRepo, "gitlab"},
		{GroupCodeRepo, "github"},
		{GroupCodeRepo, "gitee"},
		{GroupImageRepo, "harbor"},
		{GroupImageRepo, "registry"},
		{GroupArtifactRepo, "nexus"},
		{GroupArtifactRepo, "jfrog"},
		{GroupCodeScan, "sonarqube"},
		{GroupDeployTarget, "kubernetes"},
		{GroupDeployTarget, "host"},
		{GroupDeployTarget, "host_group"},
	}

	for _, tc := range cases {
		t.Run(tc.groupCode+"/"+tc.channelType, func(t *testing.T) {
			provider := providers[tc.channelType]
			if provider == nil {
				t.Fatalf("missing provider for %s", tc.channelType)
			}
			capabilities := provider.Capabilities()
			supported := map[string]bool{}
			for _, key := range capabilities.SupportedQueries {
				supported[key] = true
			}
			addressFields := map[string]bool{}
			for _, format := range capabilities.SupportedAddressFormats {
				addressFields[format.Field] = true
			}
			for _, field := range StandardVariableFields(tc.groupCode, tc.channelType) {
				if field.Provider == "" || field.UIControl == "deployConfig" {
					continue
				}
				if IsAddressVariableField(field) {
					if !supported[field.Provider] && !addressFields[field.Field] {
						t.Fatalf("field %s address format is not supported", field.Field)
					}
					continue
				}
				if !supported[field.Provider] {
					t.Fatalf("field %s provider %s is not supported", field.Field, field.Provider)
				}
			}
		})
	}
}

type fakeCapabilityProvider struct {
	queries   []string
	addresses []string
}

func (p fakeCapabilityProvider) Capabilities() ProviderCapabilities {
	formats := make([]AddressFormat, 0, len(p.addresses))
	for _, field := range p.addresses {
		formats = append(formats, AddressFormat{Field: field})
	}
	return ProviderCapabilities{SupportedQueries: p.queries, SupportedAddressFormats: formats}
}

func (p fakeCapabilityProvider) QueryOptions(context.Context, *QueryOptionsRequest) (*QueryOptionsResponse, error) {
	return nil, nil
}

func (p fakeCapabilityProvider) ResolveAddress(context.Context, *ResolveAddressRequest) (*ResolveAddressResponse, error) {
	return nil, nil
}

func (p fakeCapabilityProvider) RenderOutput(context.Context, *RenderOutputRequest) (*RenderOutputResponse, error) {
	return nil, nil
}

func (p fakeCapabilityProvider) ValidateValue(context.Context, *ValidateValueRequest) error {
	return nil
}
