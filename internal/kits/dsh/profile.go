package dsh

import (
	"encoding/json"
	"github.com/liyufei916/agent-harbor/internal/kits"
	"github.com/liyufei916/agent-harbor/internal/runtime"
	"path/filepath"
	"strconv"
)

// 使用官方插件的显式组合，避免默认 bundle 启用 shell、插件管理或宿主授权链。
func profile(input kits.KitInput, model string) ([]runtime.ConfigFile, error) {
	return buildProfile(input, model, false)
}

func buildProfile(input kits.KitInput, model string, conversation bool) ([]runtime.ConfigFile, error) {
	rows := []map[string]any{}
	add := func(id, pkg string, config map[string]any) {
		row := map[string]any{"id": id, "name": "@deepseek-ai/" + pkg}
		if config != nil {
			row["config"] = config
		}
		rows = append(rows, row)
	}
	for _, item := range [][2]string{
		{"timer", "cordis-plugin-timer"}, {"llm", "dsh-llm"}, {"session", "dsh-session"},
		{"session-log-deepseek", "dsh-session-log-deepseek"}, {"session-projection", "dsh-session-projection"},
		{"agent", "dsh-agent"}, {"tools", "dsh-tools"},
		{"working-directory", "dsh-working-directory"}, {"deepseek-llm-api-extensions", "dsh-deepseek-llm-api-extensions"},
		{"token-meter", "dsh-token-meter"}, {"jobs", "dsh-jobs-local"},
	} {
		add(item[0], item[1], nil)
	}
	add("agent-default-model", "dsh-agent-default-model", map[string]any{"provider": "deepseek-official", "model": model, "reasoningEffort": "off"})
	add("llm-deepseek", "dsh-llm-deepseek-api-key", map[string]any{"apiKeyEnv": "DEEPSEEK_API_KEY", "baseURL": input.ModelBaseURL, "thinking": "disabled", "maxTokens": 8192, "models": []any{map[string]any{"id": model, "contextWindow": 131072, "maxTokens": 8192}}, "streamIdleTimeoutMs": 30000})
	add("system-prompt", "dsh-system-prompt", map[string]any{"personaPrefix": "You are a coding agent running inside Agent Harbor. Use only the provided filesystem and harbor MCP tools. Do not execute shell commands, install plugins, or access files outside the authorized workspace.", "includeHarnessIdentity": false, "includeRuntimeContext": false})
	add("sandbox-policy", "dsh-sandbox-policy", map[string]any{"mode": "workspace-write"})
	add("fs-sandbox", "dsh-fs-sandbox", nil)
	add("tool-fs", "dsh-tool-fs", map[string]any{"read": true, "write": true, "edit": true, "readImage": false})
	add("agent-loop", "dsh-agent-loop", map[string]any{"agents": []any{}})
	urls, err := kits.MCPGateURLs(input)
	if err != nil {
		return nil, err
	}
	for i, endpoint := range urls {
		id := "connection"
		if i > 0 {
			id += "-" + strconv.Itoa(i+1)
		}
		// failOnStartupError=false：多挂载时单个 MCP 协商/启动失败不应拖垮整趟 Agent。
		add(id, "dsh-mcp-client", map[string]any{"serverName": kits.MCPServerName(input, i), "transport": "streamable-http", "url": endpoint, "headers": map[string]string{"Authorization": "Bearer " + input.Sentinel}, "failOnStartupError": false, "reconnect": map[string]any{"enabled": false}})
	}

	if conversation {
		// ConfigFiles 落盘在宿主机 PrivateHome；Docker 下 guest 只能见 /home/harbor/…
		conversationName := filepath.Join(input.PrivateHome, "harbor-conversation.mjs")
		if kits.DockerGuest() {
			conversationName = filepath.Join(kits.GuestHome, "harbor-conversation.mjs")
		}
		rows = append(rows, map[string]any{"id": "harbor-conversation", "name": conversationName})
	} else {
		add("headless-startup", "dsh-headless/startup", nil)
		add("headless-runner", "dsh-headless", map[string]any{"json": true})
	}
	raw, err := json.Marshal([]any{map[string]any{"insert": rows}})
	if err != nil {
		return nil, err
	}
	// JSON 是 YAML 子集；不用可执行 !!js 表达式，任务不进入配置。
	return []runtime.ConfigFile{
		{RelativePath: "dsh/profiles/harbor/package.json", Content: []byte(`{"name":"harbor-dsh-profile","private":true,"dsh":{"profile":{"bundles":[]}}}`)},
		{RelativePath: "dsh/profiles/harbor/cordis.yml", Content: []byte("[]\n")},
		{RelativePath: "dsh/profiles/harbor/cordis.patch.yml", Content: raw},
	}, nil
}
