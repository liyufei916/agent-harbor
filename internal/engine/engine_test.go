package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestNewSelectedDefaultsToColima(t *testing.T) {
	if _, ok := NewSelected("").(*Colima); !ok {
		t.Fatal("product default must be Colima")
	}
	if _, ok := NewSelected(SourceSystemDocker).(*SystemDocker); !ok {
		t.Fatal("system-docker source")
	}
}

func TestColimaHealthFailClosedWithoutBinaries(t *testing.T) {
	c := NewColima()
	c.Root = t.TempDir()
	c.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	st, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Ready || st.Source != SourceHarborColima {
		t.Fatalf("%+v", st)
	}
	if !strings.Contains(st.Message, "colima") {
		t.Fatalf("message=%q", st.Message)
	}
}

func TestSystemDockerHealthWithoutBinary(t *testing.T) {
	s := NewSystemDocker()
	s.lookPath = func(string) (string, error) { return "", errors.New("not found") }
	st, err := s.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Ready || st.Source != SourceSystemDocker {
		t.Fatalf("%+v", st)
	}
	if !strings.Contains(st.Message, "docker CLI") {
		t.Fatalf("message=%q", st.Message)
	}
}

func TestErrEngineCode(t *testing.T) {
	app := errEngine("x")
	if app == nil || app.Code != contracts.ErrIsolationUnavailable {
		t.Fatalf("%#v", app)
	}
}
