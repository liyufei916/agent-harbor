package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateWorkspacePath(t *testing.T) {
	dir := t.TempDir()
	res := ValidateWorkspacePath(dir)
	if res.Error != nil || res.Path != filepath.Clean(dir) {
		t.Fatalf("abs dir: %+v", res)
	}
	rel := ValidateWorkspacePath("relative/nope")
	if rel.Error == nil {
		t.Fatal("relative must fail")
	}
	cancel := ValidateWorkspacePath("")
	if !cancel.Cancelled {
		t.Fatal("empty → cancelled")
	}
}

func TestListWorkspaceShortcutsFindsDemos(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// 测试在 internal/desktop；向上找仓库 demos/workspace
	root := filepath.Clean(filepath.Join(cwd, "../.."))
	demo := filepath.Join(root, "demos", "workspace")
	if st, err := os.Stat(demo); err != nil || !st.IsDir() {
		t.Skip("no demos/workspace in tree")
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	list := listWorkspaceShortcuts()
	if len(list) == 0 {
		t.Fatal("expected demos shortcut")
	}
	if list[0].Path != filepath.Clean(demo) {
		t.Fatalf("got %q want %q", list[0].Path, demo)
	}
}

func TestHTTPLiveRPCHealthAndAuth(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	srv, err := StartHTTPLive(api, HTTPLiveConfig{Addr: "127.0.0.1:0", Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	base := srv.URL()

	// health 无需 token
	resp, err := http.Get(base + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("health status %d", resp.StatusCode)
	}

	// 无 token 拒 RPC
	bad, err := http.Post(base+"/v1/rpc", "application/json", bytes.NewReader([]byte(`{"method":"ListAgents"}`)))
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", bad.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodPost, base+"/v1/rpc", bytes.NewReader([]byte(`{"method":"GetRuntimeHealth"}`)))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	ok, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer ok.Body.Close()
	if ok.StatusCode != 200 {
		t.Fatalf("rpc status %d", ok.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(ok.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, has := body["available"]; !has {
		t.Fatalf("health body %+v", body)
	}

	// SelectWorkspace 路径
	payload, _ := json.Marshal(map[string]any{
		"method": "SelectWorkspace",
		"params": map[string]string{"path": t.TempDir()},
	})
	req2, _ := http.NewRequest(http.MethodPost, base+"/v1/rpc", bytes.NewReader(payload))
	req2.Header.Set("Authorization", "Bearer test-token")
	req2.Header.Set("Content-Type", "application/json")
	ok2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer ok2.Body.Close()
	var ws map[string]any
	_ = json.NewDecoder(ok2.Body).Decode(&ws)
	if ws["path"] == nil || ws["path"] == "" {
		t.Fatalf("workspace %+v", ws)
	}

}

func TestMaybeStartHTTPLiveOffByDefault(t *testing.T) {
	_ = os.Unsetenv("HARBOR_HTTP_LIVE")
	app := NewApp()
	api := NewDesktopAPI(app)
	s, err := MaybeStartHTTPLive(api)
	if err != nil || s != nil {
		t.Fatalf("default off: %v %v", s, err)
	}
}

func TestHTTPLiveRejectsNonLoopback(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	_, err := StartHTTPLive(api, HTTPLiveConfig{Addr: "0.0.0.0:18766", Token: "x"})
	if err == nil {
		t.Fatal("expected reject")
	}
	time.Sleep(10 * time.Millisecond)
}
