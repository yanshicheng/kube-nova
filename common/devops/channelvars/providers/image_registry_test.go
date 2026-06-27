package providers

import (
	"context"
	"testing"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

func TestHarborProviderResolveAddressUsesStandardImageFields(t *testing.T) {
	provider := NewHarborProvider(fakeImageChannelManager{channelType: "harbor"})
	resp, err := provider.ResolveAddress(context.Background(), &channelvars.ResolveAddressRequest{
		Address:   "https://harbor.example.com/public/nginx:v1",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("resolve harbor address: %v", err)
	}
	if !resp.Resolved {
		t.Fatalf("harbor address should be resolved: %s", resp.Reason)
	}
	assertField(t, resp.Fields, channelvars.FieldDynamicProject, "public")
	assertField(t, resp.Fields, channelvars.FieldDynamicImage, "nginx")
	assertField(t, resp.Fields, channelvars.FieldDynamicTag, "v1")
	assertField(t, resp.Fields, channelvars.FieldDynamicImageTag, "nginx:v1")
	assertField(t, resp.Fields, channelvars.FieldAddressProjectURL, "harbor.example.com/public")
	assertField(t, resp.Fields, channelvars.FieldAddressImageURL, "harbor.example.com/public/nginx")
	assertField(t, resp.Fields, channelvars.FieldAddressImageTagURL, "harbor.example.com/public/nginx:v1")
}

func TestRegistryProviderResolveAddressUsesStandardImageFields(t *testing.T) {
	provider := NewRegistryProvider(fakeImageChannelManager{channelType: "registry"})
	resp, err := provider.ResolveAddress(context.Background(), &channelvars.ResolveAddressRequest{
		Address:   "registry.example.com/public/nginx:v1",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("resolve registry address: %v", err)
	}
	if !resp.Resolved {
		t.Fatalf("registry address should be resolved: %s", resp.Reason)
	}
	assertField(t, resp.Fields, channelvars.FieldDynamicProject, "public")
	assertField(t, resp.Fields, channelvars.FieldDynamicImage, "nginx")
	assertField(t, resp.Fields, channelvars.FieldDynamicTag, "v1")
	assertField(t, resp.Fields, channelvars.FieldDynamicImageTag, "nginx:v1")
	assertField(t, resp.Fields, channelvars.FieldAddressProjectURL, "registry.example.com/public")
	assertField(t, resp.Fields, channelvars.FieldAddressImageURL, "registry.example.com/public/nginx")
	assertField(t, resp.Fields, channelvars.FieldAddressImageTagURL, "registry.example.com/public/nginx:v1")
}

func TestImageProviderKeepsLegacyProjectAliases(t *testing.T) {
	parentValues := map[string]string{
		channelvars.FieldAddressProjectURL: "harbor.example.com/public",
		channelvars.FieldAddressImageURL:   "harbor.example.com/public/nginx",
	}
	if got := imageProjectValue(parentValues); got != "public" {
		t.Fatalf("image project = %s, want public", got)
	}
	if got := imageNameValue(parentValues); got != "nginx" {
		t.Fatalf("image name = %s, want nginx", got)
	}
}

func assertField(t *testing.T, fields map[string]string, key, want string) {
	t.Helper()
	if got := fields[key]; got != want {
		t.Fatalf("field %s = %q, want %q", key, got, want)
	}
}

type fakeImageChannelManager struct {
	channelType string
}

func (m fakeImageChannelManager) GetChannelTypeByCode(context.Context, string) (*channelvars.ChannelType, error) {
	return nil, nil
}

func (m fakeImageChannelManager) GetInstance(context.Context, int64) (*channelvars.ChannelInstance, error) {
	return &channelvars.ChannelInstance{ID: 1, ChannelType: m.channelType, Endpoint: "https://example.com"}, nil
}

func (m fakeImageChannelManager) FindInstancesByEndpoint(context.Context, string, int64) ([]*channelvars.ChannelInstance, error) {
	return []*channelvars.ChannelInstance{{ID: 1, ChannelType: m.channelType, Endpoint: "https://example.com"}}, nil
}
