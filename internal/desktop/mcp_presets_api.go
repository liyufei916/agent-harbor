package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyufei916/agent-harbor/internal/contracts"
	"github.com/liyufei916/agent-harbor/internal/mcppresets"
)

// ListMcpPresets 返回可一键添加的预置 MCP（无密钥）。
func (d *DesktopAPI) ListMcpPresets() contracts.McpPresetsResponse {
	list := mcppresets.Catalog()
	out := make([]contracts.McpPresetInfo, 0, len(list))
	for _, p := range list {
		out = append(out, contracts.McpPresetInfo{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Kind:        p.Kind,
			NeedsAPIKey: p.NeedsAPIKey,
			Origin:      p.Origin,
		})
	}
	return contracts.McpPresetsResponse{Presets: out}
}

// AddMcpPreset 添加预置：stdio 立即连接；Context7 须带 Token。
func (d *DesktopAPI) AddMcpPreset(in contracts.AddMcpPresetInput) contracts.AccountResponse {
	id := strings.TrimSpace(in.PresetID)
	if id == "" {
		return contracts.AccountResponse{
			Error: contracts.NewAppError(contracts.ErrInvalidArgument, "请指定预置 MCP。", false),
		}
	}
	dest := filepath.Join(d.app.dataDirOrTemp(), "mcp-presets")
	launch, err := mcppresets.Prepare(id, dest, "")
	if err != nil {
		return contracts.AccountResponse{Error: mapError(err)}
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = launch.Name
	}
	var connect contracts.ConnectInput
	if launch.NeedsAPIKey {
		key := strings.TrimSpace(in.APIKey)
		if key == "" {
			return contracts.AccountResponse{
				Error: contracts.NewAppError(contracts.ErrInvalidArgument, "Context7 须填写 Token。", false),
			}
		}
		connect = contracts.ConnectInput{
			Name:      name,
			ProfileID: "custom",
			APIKey:    key,
			Origin:    launch.Origin,
		}
	} else {
		connect = contracts.ConnectInput{
			Name:      name,
			ProfileID: "custom",
			Transport: "stdio",
			Command:   launch.Command,
			Args:      launch.Args,
			APIKey:    "stdio",
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var meta contracts.AccountInfo
	err = d.app.sessionRef().WithConnection(ctx, func(call context.Context) error {
		var cerr error
		meta, cerr = d.app.conn.ConnectMcp(call, connect)
		return cerr
	})
	if err != nil {
		return contracts.AccountResponse{Error: mapError(err)}
	}
	return contracts.AccountResponse{Account: &meta}
}

func (a *App) dataDirOrTemp() string {
	if a != nil && a.dataDir != "" {
		return a.dataDir
	}
	return filepath.Join(os.TempDir(), "agent-harbor-mcp-presets")
}
