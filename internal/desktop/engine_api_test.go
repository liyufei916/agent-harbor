package desktop

import (
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/engine"
)

func TestNormalizeEngineSource(t *testing.T) {
	if got := normalizeEngineSource(""); got != engine.SourceHarborColima {
		t.Fatal(got)
	}
	if got := normalizeEngineSource("system-docker"); got != engine.SourceSystemDocker {
		t.Fatal(got)
	}
}

func TestGetContainerEngineHealthFailClosedWithoutBins(t *testing.T) {
	t.Setenv("HARBOR_ENGINE_AUTO_DOWNLOAD", "0")
	app := NewApp()
	app.setPrefs(contracts.UIPrefs{EngineSource: engine.SourceHarborColima})
	api := &DesktopAPI{app: app}
	got := api.GetContainerEngineHealth()
	if got.Ready {
		t.Fatalf("%+v", got)
	}
	if got.Source != engine.SourceHarborColima {
		t.Fatalf("%+v", got)
	}
	if got.Message == "" {
		t.Fatal("want message")
	}
}
