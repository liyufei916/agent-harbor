package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Colima is the product-primary managed engine (B→C). M2 will download/start Colima;
// until then Ensure/Start/Health fail closed with a fixed message.
type Colima struct {
	Root string // Application Support/.../engine; empty → defaultRoot()
}

func NewColima() *Colima {
	return &Colima{}
}

func (c *Colima) root() string {
	if r := strings.TrimSpace(c.Root); r != "" {
		return r
	}
	return defaultEngineRoot()
}

func defaultEngineRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "AgentHarbor", "engine")
	}
	return filepath.Join(home, "Library", "Application Support", "AgentHarbor", "engine")
}

func (c *Colima) Ensure(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_ = c.root()
	return errEngine("Harbor 托管 Colima 尚未实现（M2）。开发可暂用本机 Docker。")
}

func (c *Colima) Start(ctx context.Context) error {
	return c.Ensure(ctx)
}

func (c *Colima) Stop(context.Context) error {
	return nil
}

func (c *Colima) Health(ctx context.Context) (Status, error) {
	err := c.Ensure(ctx)
	msg := "Harbor 托管 Colima 尚未实现（M2）。"
	if err != nil {
		msg = err.Error()
	}
	return Status{Ready: false, Source: SourceHarborColima, Message: msg}, nil
}

var _ Engine = (*Colima)(nil)
