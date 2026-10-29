package runtime

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func runtimeError(code string) *contracts.AppError {
	message := "隔离运行失败。"
	switch code {
	case "INVALID_ARGUMENT":
		message = "运行参数不合法。"
	case "WORKSPACE_DENIED":
		message = "工作区未通过隔离校验。"
	case "AGENT_UNSUPPORTED":
		message = "Agent 启动规格不受支持。"
	case "ISOLATION_UNAVAILABLE":
		message = "操作系统隔离不可用，已拒绝启动。"
	case "CLEANUP_FAILED":
		message = "隔离资源清理未确认完成。"
	case "CANCELLED":
		message = "操作已取消。"
	}
	return &contracts.AppError{Code: code, Message: message}
}

var permittedEnv = map[string]bool{
	"DSH_HOME": true, "DSH_TELEMETRY_DISABLED": true, "DEEPSEEK_API_KEY": true,
	"CODEX_HOME": true, "OPENAI_API_KEY": true,
	"ANTHROPIC_API_KEY": true, "ANTHROPIC_AUTH_TOKEN": true, "ANTHROPIC_BASE_URL": true,
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true, "CLAUDE_CODE_SIMPLE": true,
	"DISABLE_AUTOUPDATER": true, "DISABLE_TELEMETRY": true, "DISABLE_ERROR_REPORTING": true,
}

func launchEnvironment(home, tmpRoot string, extra map[string]string) ([]string, error) {
	if tmpRoot == "" {
		tmpRoot = home
	}
	path := "/usr/bin:/bin"
	if extra["CODEX_HOME"] == filepath.Join(tmpRoot, "codex") || extra["CODEX_HOME"] == filepath.Join(home, "codex") {
		// 仅 Codex 受控 profile 需要宿主工具链；process-exec 仍由 Seatbelt 白名单约束。
		path = "/usr/bin:/bin:/opt/homebrew/bin:/usr/local/bin"
	}
	tmp := filepath.Join(tmpRoot, "tmp")
	result := []string{"HOME=" + home, "TMPDIR=" + tmp, "PATH=" + path, "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8"}
	result = append(result, "CLAUDE_CODE_TMPDIR="+tmp)
	keys := make([]string, 0, len(extra))
	for key, value := range extra {
		if key == "DSH_HOME" && value != filepath.Join(tmpRoot, "dsh") && value != filepath.Join(home, "dsh") || key == "DSH_TELEMETRY_DISABLED" && value != "1" {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
		if key == "CODEX_HOME" && value != filepath.Join(tmpRoot, "codex") && value != filepath.Join(home, "codex") {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
		if !permittedEnv[key] || strings.ContainsRune(value, 0) || len(value) > 32768 {
			return nil, runtimeError("INVALID_ARGUMENT")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+extra[key])
	}
	return result, nil
}

func validateConfigFiles(files []ConfigFile) error {
	seen := make(map[string]bool)
	total := 0
	for _, file := range files {
		path := file.RelativePath
		if !filepath.IsLocal(path) || filepath.Clean(path) != path || path == "." || strings.ContainsAny(path, "\\\x00") {
			return runtimeError("INVALID_ARGUMENT")
		}
		if seen[path] {
			return runtimeError("INVALID_ARGUMENT")
		}
		seen[path] = true
		total += len(file.Content)
		if total > 1<<20 {
			return runtimeError("INVALID_ARGUMENT")
		}
	}
	for path := range seen {
		for parent := filepath.Dir(path); parent != "."; parent = filepath.Dir(parent) {
			if seen[parent] {
				return runtimeError("INVALID_ARGUMENT")
			}
		}
	}
	return nil
}

// 私有 HOME 在首次启动前仅由 Runtime 持有；拒绝任何已有节点和符号链接。
func writeConfigFiles(home string, files []ConfigFile) error {
	if err := validateConfigFiles(files); err != nil {
		return err
	}
	for _, file := range files {
		path := filepath.Join(home, file.RelativePath)
		parts := strings.Split(filepath.Dir(file.RelativePath), string(filepath.Separator))
		parent := home
		for _, part := range parts {
			if part == "." {
				continue
			}
			parent = filepath.Join(parent, part)
			info, err := os.Lstat(parent)
			if os.IsNotExist(err) {
				if err := os.Mkdir(parent, 0700); err != nil {
					return runtimeError("INVALID_ARGUMENT")
				}
				continue
			}
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
				return runtimeError("INVALID_ARGUMENT")
			}
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return runtimeError("INVALID_ARGUMENT")
		}
		_, writeErr := out.Write(file.Content)
		closeErr := out.Close()
		if writeErr != nil || closeErr != nil {
			return runtimeError("INVALID_ARGUMENT")
		}
	}
	return nil
}

// 跨 Read 保留不完整 UTF-8，不按行读取，避免超长行卡住输出。
func readOutput(reader io.Reader, stream string, output func(contracts.OutputData)) {
	buffer := make([]byte, 16<<10)
	pending := make([]byte, 0, 4)
	for {
		n, err := reader.Read(buffer)
		data := append(pending, buffer[:n]...)
		pending = nil
		end := 0
		for end < len(data) {
			if !utf8.FullRune(data[end:]) {
				break
			}
			_, size := utf8.DecodeRune(data[end:])
			end += size
		}
		emitUTF8(string(data[:end]), stream, output)
		if end < len(data) {
			pending = append(pending, data[end:]...)
		}
		if err != nil {
			if len(pending) > 0 {
				emitUTF8(strings.ToValidUTF8(string(pending), "�"), stream, output)
			}
			return
		}
	}
}

func emitUTF8(text, stream string, output func(contracts.OutputData)) {
	text = strings.ToValidUTF8(text, "�")
	for len(text) > 0 {
		n := min(len(text), 16<<10)
		for n < len(text) && !utf8.RuneStart(text[n]) {
			n--
		}
		if output != nil {
			output(contracts.OutputData{Stream: stream, Text: text[:n]})
		}
		text = text[n:]
	}
}
