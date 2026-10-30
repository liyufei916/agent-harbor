package desktop

import (
	"reflect"
	"testing"
)

func TestDesktopExposesOnlyContractMethods(t *testing.T) {
	typ := reflect.TypeOf((*DesktopAPI)(nil))
	want := map[string]bool{
		"ListAgents": true, "SelectWorkspace": true, "ConnectModel": true, "ConnectMcp": true,
		"ListMcpPresets": true, "AddMcpPreset": true,
		"ListAccounts": true, "RemoveAccount": true,
		"CreateSession": true, "GetSession": true, "StopSession": true, "StartSession": true,
		"ListSessions": true, "DestroySession": true, "SyncBackWorkspace": true,
		"GetRuntimeHealth": true, "GetContainerEngineHealth": true, "EnsureContainerEngine": true, "StopContainerEngine": true,
		"GetPermissionOverview": true,
		"ListRememberedHosts": true, "RevokeRememberedHost": true, "ResolvePermission": true,
		"PromptPermission": true,
		"GetUIPrefs":       true, "SetUIPrefs": true,
		"SwitchModel": true, "StartMcpOAuth": true, "CancelMcpOAuth": true,
		"WritePty": true, "ResizePty": true, "SaveTextFile": true,
		"RestartApp": true,
	}
	if typ.NumMethod() != len(want) {
		t.Fatalf("绑定方法数=%d", typ.NumMethod())
	}
	for i := 0; i < typ.NumMethod(); i++ {
		if !want[typ.Method(i).Name] {
			t.Fatalf("意外绑定 %s", typ.Method(i).Name)
		}
	}
}
