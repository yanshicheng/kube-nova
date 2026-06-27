package providers

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

func TestRenderHostConfigFormats(t *testing.T) {
	hosts := []hostAccessConfig{
		{Host: "192.168.1.1", Port: 22, Username: "root", Password: "password"},
		{Host: "192.168.1.2", Port: 22, Username: "root", Password: "password"},
	}
	jsonValue, err := renderHostConfigs(hosts, "json")
	if err != nil {
		t.Fatalf("render json: %v", err)
	}
	if !strings.Contains(jsonValue, `"host": "192.168.1.1"`) {
		t.Fatalf("json output should contain host, got %s", jsonValue)
	}
	jsonBase64, err := renderHostConfigs(hosts, "jsonbase64")
	if err != nil {
		t.Fatalf("render jsonBase64: %v", err)
	}
	decodedJSON, err := base64.StdEncoding.DecodeString(jsonBase64)
	if err != nil {
		t.Fatalf("decode jsonBase64: %v", err)
	}
	if string(decodedJSON) != jsonValue {
		t.Fatalf("jsonBase64 decoded value mismatch")
	}
	ansibleValue, err := renderHostConfigs(hosts, "ansible")
	if err != nil {
		t.Fatalf("render ansible: %v", err)
	}
	if !strings.Contains(ansibleValue, "[all]") || !strings.Contains(ansibleValue, "ansible_ssh_host=192.168.1.2") {
		t.Fatalf("ansible output invalid: %s", ansibleValue)
	}
	ansibleBase64, err := renderHostConfigs(hosts, "ansiblebase64")
	if err != nil {
		t.Fatalf("render ansibleBase64: %v", err)
	}
	decodedAnsible, err := base64.StdEncoding.DecodeString(ansibleBase64)
	if err != nil {
		t.Fatalf("decode ansibleBase64: %v", err)
	}
	if string(decodedAnsible) != ansibleValue {
		t.Fatalf("ansibleBase64 decoded value mismatch")
	}
}

func TestHostProviderResolveAddress(t *testing.T) {
	manager := fakeHostChannelManager{}
	provider := NewHostProvider(manager)
	resp, err := provider.ResolveAddress(context.Background(), &channelvars.ResolveAddressRequest{
		Address: `{"host":"192.168.1.1","port":22,"username":"root","password":"password"}`,
	})
	if err != nil {
		t.Fatalf("resolve address: %v", err)
	}
	if !resp.Resolved || resp.ChannelInstanceID != 10 {
		t.Fatalf("unexpected resolve response: %#v", resp)
	}
	if resp.Fields[channelvars.FieldAddressHostConfig] == "" {
		t.Fatalf("resolve response should keep address field")
	}
}

type fakeHostChannelManager struct{}

func (fakeHostChannelManager) GetChannelTypeByCode(context.Context, string) (*channelvars.ChannelType, error) {
	return nil, nil
}

func (fakeHostChannelManager) GetInstance(context.Context, int64) (*channelvars.ChannelInstance, error) {
	return nil, nil
}

func (fakeHostChannelManager) FindInstancesByEndpoint(context.Context, string, int64) ([]*channelvars.ChannelInstance, error) {
	return []*channelvars.ChannelInstance{{ID: 10, ChannelType: "host", Endpoint: "192.168.1.1"}}, nil
}
