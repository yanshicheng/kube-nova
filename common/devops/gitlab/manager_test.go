package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestHasGitLabCredentialAllowsUsernamePassword(t *testing.T) {
	if !hasGitLabCredential(devopstypes.Request{
		Credential: &devopstypes.Credential{
			Type:     "username_password",
			Username: "zhangsan",
			Password: "password",
		},
	}) {
		t.Fatal("username_password credential should be allowed")
	}
	if !hasGitLabCredential(devopstypes.Request{
		Channel: devopstypes.Channel{
			AuthType: "username_password",
			Username: "zhangsan",
			Password: "password",
		},
	}) {
		t.Fatal("channel username_password credential should be allowed")
	}
}

func TestGitLabAPIAuthUsesPasswordAsPrivateToken(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://git.example.com/api/v4/user", http.NoBody)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	applyGitLabAPIAuth(request, devopstypes.Request{
		Credential: &devopstypes.Credential{
			Type:     "username_password",
			Username: "zhangsan",
			Password: "glpat-token",
		},
	})
	if got := request.Header.Get("PRIVATE-TOKEN"); got != "glpat-token" {
		t.Fatalf("expected PRIVATE-TOKEN glpat-token, got %s", got)
	}
	if got := request.Header.Get("Authorization"); got != "" {
		t.Fatalf("GitLab API should not use basic auth, got %s", got)
	}
}

func TestListBranchesFallbackToProjectID(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/api/v4/projects":
					if r.URL.Query().Get("search") != "devops-shared-library" {
						return newTestResponse(http.StatusBadRequest, `{"message":"bad search"}`), nil
					}
					payload, _ := json.Marshal([]map[string]any{
						{
							"id":                  42,
							"name":                "devops-shared-library",
							"path":                "devops-shared-library",
							"path_with_namespace": "ikubeops/devops-shared-library",
						},
					})
					return newTestResponse(http.StatusOK, string(payload)), nil
				case "/api/v4/projects/42/repository/branches":
					payload, _ := json.Marshal([]map[string]any{
						{"name": "main"},
						{"name": "release"},
					})
					return newTestResponse(http.StatusOK, string(payload)), nil
				default:
					return newTestResponse(http.StatusBadRequest, `{"message":"unexpected path"}`), nil
				}
			}),
		}
	}

	items, total, err := Manager{}.ListBranches(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com"},
	}, "ikubeops/devops-shared-library", "", 1, 20)
	if err != nil {
		t.Fatalf("ListBranches returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Value != "main" || items[1].Value != "release" {
		t.Fatalf("unexpected items: %v", items)
	}
}

func TestListBranchesFallbackToProjectIDOnServerError(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/api/v4/projects":
					payload, _ := json.Marshal([]map[string]any{
						{
							"id":                  42,
							"name":                "devops-shared-library",
							"path":                "devops-shared-library",
							"path_with_namespace": "ikubeops/devops-shared-library",
						},
					})
					return newTestResponse(http.StatusOK, string(payload)), nil
				case "/api/v4/projects/42/repository/branches":
					payload, _ := json.Marshal([]map[string]any{{"name": "main"}})
					return newTestResponse(http.StatusOK, string(payload)), nil
				default:
					return newTestResponse(http.StatusBadRequest, `{"message":"unexpected path"}`), nil
				}
			}),
		}
	}

	items, _, err := Manager{}.ListBranches(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com"},
	}, "ikubeops/devops-shared-library", "", 1, 20)
	if err != nil {
		t.Fatalf("ListBranches returned error: %v", err)
	}
	if len(items) != 1 || items[0].Value != "main" {
		t.Fatalf("unexpected items: %v", items)
	}
}

func TestListBranchesFallbackToProjectPathWhenSavedIDFails(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Path == "/api/v4/projects/2/repository/branches":
					return newTestResponse(http.StatusInternalServerError, `500 Internal Server Error`), nil
				case r.URL.Path == "/api/v4/projects/2":
					payload, _ := json.Marshal(map[string]any{
						"id":                  2,
						"name":                "test-vue",
						"path":                "test-vue",
						"path_with_namespace": "ikubeops/test-vue",
					})
					return newTestResponse(http.StatusOK, string(payload)), nil
				case r.URL.EscapedPath() == "/api/v4/projects/ikubeops%2Ftest-vue/repository/branches":
					payload, _ := json.Marshal([]map[string]any{{"name": "main"}})
					return newTestResponse(http.StatusOK, string(payload)), nil
				default:
					return newTestResponse(http.StatusBadRequest, `{"message":"unexpected path"}`), nil
				}
			}),
		}
	}

	items, _, err := Manager{}.ListBranches(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com"},
	}, "2", "", 1, 20)
	if err != nil {
		t.Fatalf("ListBranches returned error: %v", err)
	}
	if len(items) != 1 || items[0].Value != "main" {
		t.Fatalf("unexpected items: %v", items)
	}
}

func TestListBranchesFallbackToGitHTTPRefs(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Path == "/api/v4/projects/2/repository/branches":
					return newTestResponse(http.StatusInternalServerError, "500 Internal Server Error"), nil
				case r.URL.Path == "/api/v4/projects/2":
					payload, _ := json.Marshal(map[string]any{
						"id":                  2,
						"name":                "test-vue",
						"path":                "test-vue",
						"path_with_namespace": "ikubeops/test-vue",
						"http_url_to_repo":    "https://wrong.example.com/ikubeops/test-vue.git",
					})
					return newTestResponse(http.StatusOK, string(payload)), nil
				case r.URL.Path == "/ikubeops/test-vue.git/info/refs":
					if r.URL.Host != "git.example.com:30081" {
						return newTestResponse(http.StatusBadRequest, `{"message":"bad host"}`), nil
					}
					if r.URL.Query().Get("service") != "git-upload-pack" {
						return newTestResponse(http.StatusBadRequest, `{"message":"bad service"}`), nil
					}
					return newTestResponse(http.StatusOK, string(gitUploadPackPayload(
						"refs/heads/main",
						"refs/heads/release",
						"refs/tags/v1.0.0",
					))), nil
				default:
					return newTestResponse(http.StatusBadRequest, `{"message":"unexpected path"}`), nil
				}
			}),
		}
	}

	items, total, err := Manager{}.ListBranches(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com:30081"},
	}, "2", "", 1, 20)
	if err != nil {
		t.Fatalf("ListBranches returned error: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 branches, got total=%d items=%v", total, items)
	}
	if items[0].Value != "main" || items[1].Value != "release" {
		t.Fatalf("unexpected branches: %v", items)
	}
}

func gitUploadPackPayload(refs ...string) []byte {
	lines := [][]byte{
		gitPktLine("# service=git-upload-pack\n"),
		[]byte("0000"),
	}
	for index, ref := range refs {
		suffix := "\n"
		if index == 0 {
			suffix = "\x00multi_ack\n"
		}
		lines = append(lines, gitPktLine("0000000000000000000000000000000000000000 "+ref+suffix))
	}
	lines = append(lines, []byte("0000"))
	return []byte(strings.Join(byteSlicesToStrings(lines), ""))
}

func gitPktLine(payload string) []byte {
	size := len(payload) + 4
	return []byte(fmt.Sprintf("%04x%s", size, payload))
}

func byteSlicesToStrings(items [][]byte) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, string(item))
	}
	return result
}

func TestGitlabRefAPIUsesEscapedProjectPath(t *testing.T) {
	api := gitlabRefAPI("group/repo", "branches", "page=1")
	expected := "/api/v4/projects/group%2Frepo/repository/branches?page=1"
	if api != expected {
		t.Fatalf("expected %s, got %s", expected, api)
	}
}

func TestFindProjectIDNoMatch(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				payload, _ := json.Marshal([]map[string]any{
					{"id": 7, "path": "other", "name": "other", "path_with_namespace": "other/repo"},
				})
				return newTestResponse(http.StatusOK, string(payload)), nil
			}),
		}
	}

	id, err := findProjectID(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com"},
	}, "missing/repo")
	if err != nil {
		t.Fatalf("findProjectID returned error: %v", err)
	}
	if id != "" {
		t.Fatalf("expected empty id, got %s", id)
	}
}

func TestFindProjectIDFallbackByPath(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				payload, _ := json.Marshal([]map[string]any{
					{"id": 42, "path": "devops-shared-library", "name": "devops-shared-library"},
				})
				return newTestResponse(http.StatusOK, string(payload)), nil
			}),
		}
	}

	id, err := findProjectID(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com"},
	}, "ikubeops/devops-shared-library")
	if err != nil {
		t.Fatalf("findProjectID returned error: %v", err)
	}
	if id != "42" {
		t.Fatalf("expected id 42, got %s", id)
	}
}

func TestFindProjectIDNonSuccess(t *testing.T) {
	oldFactory := gitlabClientFactory
	defer func() { gitlabClientFactory = oldFactory }()

	gitlabClientFactory = func(req devopstypes.Request) *http.Client {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return newTestResponse(http.StatusInternalServerError, `{"message":"boom"}`), nil
			}),
		}
	}

	_, err := findProjectID(context.Background(), devopstypes.Request{
		Channel: devopstypes.Channel{Type: "gitlab", Endpoint: "https://git.example.com"},
	}, "missing/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != fmt.Sprintf("GitLab 项目列表 HTTP %d: boom", http.StatusInternalServerError) {
		t.Fatalf("unexpected error: %s", got)
	}
}
