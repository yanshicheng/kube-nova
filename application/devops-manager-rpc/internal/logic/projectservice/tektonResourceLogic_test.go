package projectservicelogic

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseTektonManagedResourcesAcceptValidKind(t *testing.T) {
	if _, err := parseConfigMap(`apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-config
data:
  key: value
`); err != nil {
		t.Fatalf("parse ConfigMap failed: %v", err)
	}

	if _, err := parsePVC(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: demo-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
`); err != nil {
		t.Fatalf("parse PVC failed: %v", err)
	}

	if _, err := parseServiceAccount(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: demo-sa
`); err != nil {
		t.Fatalf("parse ServiceAccount failed: %v", err)
	}

	if _, err := parseSecret(`apiVersion: v1
kind: Secret
metadata:
  name: demo-secret
stringData:
  token: abc
`, "project-1", "binding-1"); err != nil {
		t.Fatalf("parse Secret failed: %v", err)
	}
}

func TestParseTektonManagedResourceRejectWrongKind(t *testing.T) {
	_, err := parseConfigMap(`apiVersion: v1
kind: Secret
metadata:
  name: demo
`)
	if err == nil {
		t.Fatal("expected kind validation error")
	}
	if !strings.Contains(err.Error(), "YAML kind 必须是 ConfigMap") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseTektonManagedResourceRejectWrongAPIVersion(t *testing.T) {
	_, err := parseSecret(`apiVersion: apps/v1
kind: Secret
metadata:
  name: demo
`, "project-1", "binding-1")
	if err == nil {
		t.Fatal("expected apiVersion validation error")
	}
	if !strings.Contains(err.Error(), "Secret apiVersion 必须是 v1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarshalCoreResourceYAMLIncludesTypeMeta(t *testing.T) {
	cases := []struct {
		name    string
		content string
		kind    string
	}{
		{
			name:    "configMap",
			content: marshalCoreConfigMapYAML(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}),
			kind:    "ConfigMap",
		},
		{
			name: "pvc",
			content: marshalCorePVCYAML(&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "demo"},
			}),
			kind: "PersistentVolumeClaim",
		},
		{
			name: "serviceAccount",
			content: marshalCoreServiceAccountYAML(&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "demo"},
			}),
			kind: "ServiceAccount",
		},
		{
			name: "secret",
			content: marshalCoreSecretYAML(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "demo"},
			}),
			kind: "Secret",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.content, "apiVersion: v1") {
				t.Fatalf("YAML missing apiVersion: %s", tc.content)
			}
			if !strings.Contains(tc.content, "kind: "+tc.kind) {
				t.Fatalf("YAML missing kind %s: %s", tc.kind, tc.content)
			}
		})
	}
}

func TestPodFromYAMLFillsTypeMeta(t *testing.T) {
	item, err := podFromYAML(`metadata:
  name: demo-pod
  namespace: ci
spec:
  containers:
    - name: step
      image: busybox
status:
  phase: Succeeded
`)
	if err != nil {
		t.Fatalf("podFromYAML failed: %v", err)
	}
	if item.kind != "Pod" {
		t.Fatalf("kind = %q, want Pod", item.kind)
	}
	if item.apiVersion != "v1" {
		t.Fatalf("apiVersion = %q, want v1", item.apiVersion)
	}
	if !strings.Contains(item.yaml, "kind: Pod") {
		t.Fatalf("YAML missing kind: %s", item.yaml)
	}
}
