package desktop

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestListMcpPresets(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	res := api.ListMcpPresets()
	if res.Error != nil || len(res.Presets) < 3 {
		t.Fatalf("%+v", res)
	}
}

func TestAddMcpPresetTime(t *testing.T) {
	// 内存凭据库：避免本机 Keychain 不可用时 NewAppWithDataDir 持久化 Put 失败。
	app := NewApp()
	app.SetSessionService(&recordingSession{})
	api := NewDesktopAPI(app)
	res := api.AddMcpPreset(contracts.AddMcpPresetInput{PresetID: "harbor-time"})
	if res.Error != nil {
		t.Fatalf("code=%s msg=%s", res.Error.Code, res.Error.Message)
	}
	if res.Account == nil || res.Account.Transport != "stdio" {
		t.Fatalf("%+v", res.Account)
	}
}

func TestAddMcpPresetContext7NeedsKey(t *testing.T) {
	app := NewApp()
	api := NewDesktopAPI(app)
	res := api.AddMcpPreset(contracts.AddMcpPresetInput{PresetID: "context7"})
	if res.Error == nil || res.Error.Code != contracts.ErrInvalidArgument {
		t.Fatalf("%+v", res)
	}
}
