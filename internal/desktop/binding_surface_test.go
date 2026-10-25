package desktop

import (
	"reflect"
	"testing"
)

func TestDesktopExposesOnlyContractMethods(t *testing.T) {
	typ := reflect.TypeOf((*DesktopAPI)(nil))
	want := map[string]bool{
		"ListAgents": true, "SelectWorkspace": true, "ConnectModel": true, "ConnectMcp": true,
		"ListAccounts": true, "RemoveAccount": true,
		"CreateSession": true, "GetSession": true, "StopSession": true,
		"ListSessions": true, "DestroySession": true,
		"GetRuntimeHealth": true, "GetPermissionOverview": true,
		"ListRememberedHosts": true, "RevokeRememberedHost": true, "ResolvePermission": true,
		"GetUIPrefs": true, "SetUIPrefs": true,
		"SwitchModel": true, "StartMcpOAuth": true, "CancelMcpOAuth": true,
		"WritePty": true, "ResizePty": true,
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
