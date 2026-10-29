package runtime

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/liyufei916/agent-harbor/internal/contracts"
)

func TestConfigPathsAndPermissions(t *testing.T) {
	for _, path := range []string{"../escape", "/absolute", "a/../escape", ".", "a//b", "a\\b", "a\x00b"} {
		if validateConfigFiles([]ConfigFile{{RelativePath: path}}) == nil {
			t.Fatalf("接受越界或歧义路径 %q", path)
		}
	}
	for _, files := range [][]ConfigFile{
		{{RelativePath: "a"}, {RelativePath: "a"}}, {{RelativePath: "a"}, {RelativePath: "a/b"}},
	} {
		if validateConfigFiles(files) == nil {
			t.Fatal("接受冲突配置路径")
		}
	}
	home := t.TempDir()
	files := []ConfigFile{{RelativePath: "config/mcp.json", Content: []byte("fixture")}}
	if err := writeConfigFiles(home, files); err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{"config": 0700, "config/mcp.json": 0600} {
		info, err := os.Stat(filepath.Join(home, path))
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("权限错误 %s %v", path, err)
		}
	}
	if writeConfigFiles(home, files) == nil {
		t.Fatal("覆盖了已有配置")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, "link")); err != nil {
		t.Fatal(err)
	}
	if writeConfigFiles(home, []ConfigFile{{RelativePath: "link/escape"}}) == nil {
		t.Fatal("沿符号链接写出 HOME")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape")); !os.IsNotExist(err) {
		t.Fatal("越界文件被创建")
	}
}

func TestEnvironmentAllowsOnlyKitInputs(t *testing.T) {
	for _, key := range []string{"HOME", "PATH", "TMPDIR", "CLAUDE_CODE_TMPDIR", "DYLD_INSERT_LIBRARIES", "NODE_OPTIONS", "SSH_AUTH_SOCK", "AWS_SECRET_ACCESS_KEY"} {
		if _, err := launchEnvironment("/private/home", "", map[string]string{key: "value"}); err == nil {
			t.Fatalf("接受危险环境变量 %s", key)
		}
	}
	env, err := launchEnvironment("/private/home", "", map[string]string{"ANTHROPIC_API_KEY": "test-sentinel"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(env, "\n"), "HOME=/private/home\n") {
		t.Fatal("未配置私有 HOME")
	}
}

func TestCodexEnvironmentGetsToolchainPathOnlyWithPrivateCodexHome(t *testing.T) {
	env, err := launchEnvironment("/private/home", "", map[string]string{"CODEX_HOME": "/private/home/codex"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "PATH=/usr/bin:/bin:/opt/homebrew/bin:/usr/local/bin") {
		t.Fatalf("Codex 未获得受控工具链 PATH: %v", env)
	}
}

func TestLaunchEnvironmentProjectHomeKeepsPrivateTmp(t *testing.T) {
	env, err := launchEnvironment("/private/workspace", "/private/home", map[string]string{"ANTHROPIC_API_KEY": "test-sentinel"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "HOME=/private/workspace\n") {
		t.Fatalf("HOME 应为工作区: %v", env)
	}
	if !strings.Contains(joined, "TMPDIR=/private/home/tmp\n") || !strings.Contains(joined, "CLAUDE_CODE_TMPDIR=/private/home/tmp\n") {
		t.Fatalf("临时目录须留在私有 HOME: %v", env)
	}
}

type byteReader struct{ input []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.input) == 0 {
		return 0, io.EOF
	}
	p[0], r.input = r.input[0], r.input[1:]
	return 1, nil
}

func TestOutputDecoderPreservesSplitUTF8(t *testing.T) {
	input := "首尾中文🙂" + strings.Repeat("x", 20000)
	var output strings.Builder
	readOutput(&byteReader{input: []byte(input)}, "stdout", func(data contracts.OutputData) {
		if !utf8.ValidString(data.Text) || len(data.Text) > 16<<10 {
			t.Fatal("输出不是受限 UTF-8 块")
		}
		output.WriteString(data.Text)
	})
	if output.String() != input {
		t.Fatal("跨块解码破坏了文本")
	}
	output.Reset()
	readOutput(bytes.NewReader([]byte{0xe4}), "stderr", func(data contracts.OutputData) { output.WriteString(data.Text) })
	if output.String() != "�" {
		t.Fatal("不完整 UTF-8 尾部未替换")
	}
}
