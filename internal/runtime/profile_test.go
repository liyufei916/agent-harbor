package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyInstallationHasNoRecursiveExecution(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "installation")
	if err := os.Mkdir(install, 0700); err != nil {
		t.Fatal(err)
	}
	profile, err := makeProfile(root, root, []string{install}, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	quoted, _ := sbQuote(install)
	if !strings.Contains(profile, "(allow file-read* file-map-executable (subpath "+quoted+"))") {
		t.Fatal("缺少安装目录只读授权")
	}
	if strings.Contains(profile, "(allow process-exec (subpath") || strings.Contains(profile, "(allow process-exec (literal "+quoted) {
		t.Fatal("目录不应授予执行权")
	}
	withDesc, err := makeProfile(root, root, []string{install}, nil, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withDesc, "(allow process-fork)") || !strings.Contains(withDesc, "(allow process-exec (subpath "+quoted+")") {
		t.Fatal("允许后代时应授权安装目录 process-exec")
	}
	if !strings.Contains(withDesc, "(allow file-ioctl)") || !strings.Contains(withDesc, "/dev/ttys") {
		t.Fatal("允许后代时应授权 PTY tty ioctl（setRawMode）")
	}
	strict, err := makeProfile(root, root, []string{install}, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strict, "(allow file-ioctl)") {
		t.Fatal("严格单进程不应开放 file-ioctl")
	}
}

func TestCodexProfileAllowsOnlyManagedRequirementFiles(t *testing.T) {
	profile, err := makeProfile(t.TempDir(), t.TempDir(), nil, nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range codexManagedConfigPaths {
		quoted, _ := sbQuote(path)
		if !strings.Contains(profile, "(allow file-read* (literal "+quoted+"))") {
			t.Fatalf("Codex profile 未允许固定 managed 文件: %s", path)
		}
	}
	for _, service := range []string{"com.apple.cfprefsd.daemon", "com.apple.cfprefsd.agent"} {
		if !strings.Contains(profile, "(allow mach-lookup (global-name \""+service+"\"))") {
			t.Fatalf("Codex profile 未允许最小偏好服务查询: %s", service)
		}
	}
	if !strings.Contains(profile, `(allow mach-lookup (local-name "com.apple.cfprefsd.agent"))`) {
		t.Fatal("Codex profile 未允许 cfprefsd local-name")
	}
	if !strings.Contains(profile, `(allow user-preference-read)`) {
		t.Fatal("Codex profile 未允许 user-preference-read")
	}
	if !strings.Contains(profile, `(allow ipc-posix-shm-read* (ipc-posix-name-prefix "apple.cfprefs."))`) {
		t.Fatal("Codex profile 未允许 apple.cfprefs POSIX shm 读")
	}
	if strings.Contains(profile, "(allow user-preference-write)") {
		t.Fatal("Codex profile 不应开放 user-preference-write")
	}
	if strings.Contains(profile, "(allow file-read* (subpath \"/etc\"))") {
		t.Fatal("Codex profile 不应开放整个 /etc")
	}
	if !strings.Contains(profile, "(allow process-fork)") {
		t.Fatal("Codex 受控子进程应允许 process-fork")
	}
	if strings.Contains(profile, "(allow file-ioctl)") {
		t.Fatal("仅 AllowChildProcesses 不应开放 PTY ioctl")
	}
}

func TestSeatbeltProfileFailClosedShape(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	ws := filepath.Join(root, "ws")
	for _, p := range []string{home, ws} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := makeProfile(ws, home, []string{"/usr/bin/true"}, []string{"127.0.0.1:41001"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(profile, "(deny default)") {
		t.Fatal("缺少 deny default")
	}
	if strings.Contains(profile, "(allow default)") {
		t.Fatal("禁止 allow default（fail-open）")
	}
	if strings.Contains(profile, `import "system.sb"`) {
		t.Fatal("生产 profile 不得导入 system.sb")
	}
	if !strings.Contains(profile, "(deny process-fork)") {
		t.Fatal("严格模式应 deny process-fork")
	}
	if !strings.Contains(profile, `(allow network-outbound (remote tcp4 "localhost:41001"))`) {
		t.Fatal("出站须限定 loopback 端口")
	}
	for _, bad := range []string{
		`(allow network-outbound)`,
		`(allow file-read* (subpath "/Users"))`,
		`(allow file-read* (subpath "/etc"))`,
		`(allow file-read* (subpath "/private"))`,
	} {
		if strings.Contains(profile, bad) {
			t.Fatalf("越权规则出现在 profile: %s", bad)
		}
	}
	quotedWS, _ := sbQuote(ws)
	quotedHome, _ := sbQuote(home)
	if !strings.Contains(profile, "(allow file-read* file-write* (subpath "+quotedWS+"))") {
		t.Fatal("工作区读写缺失")
	}
	if !strings.Contains(profile, "(allow file-read* file-write* (subpath "+quotedHome+"))") {
		t.Fatal("私有 HOME 读写缺失")
	}
}

func TestDSHEnvironmentCannotEscapePrivateHome(t *testing.T) {
	if _, err := launchEnvironment("/private/home", "", map[string]string{"DSH_HOME": "/Users/host/.dsh"}); err == nil {
		t.Fatal("允许借用宿主 HOME")
	}
	if _, err := launchEnvironment("/private/home", "", map[string]string{"DSH_TELEMETRY_DISABLED": ""}); err == nil {
		t.Fatal("允许关闭遥测禁用约束")
	}
}

func TestEndpointPortAcceptsOnlyLoopbackTCPPorts(t *testing.T) {
	valid := []string{"127.0.0.1:1", "127.0.0.1:41001", "127.0.0.1:65535"}
	for _, address := range valid {
		if port, err := endpointPort(address); err != nil || port < 1 {
			t.Fatalf("有效地址被拒绝: %s, %v", address, err)
		}
	}
	invalid := []string{
		"localhost:41001", "127.0.0.2:41001", "0.0.0.0:41001",
		"127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:041001",
		"127.0.0.1", "[::1]:41001", "127.0.0.1:41001:extra",
	}
	for _, address := range invalid {
		if _, err := endpointPort(address); err == nil {
			t.Fatalf("危险或非法地址被接受: %s", address)
		}
	}
}

func TestCanonicalPathResolvesSymlinksAndRejectsMissing(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalPath(link)
	if err != nil || got != want {
		t.Fatalf("symlink=%q err=%v want=%q", got, err, want)
	}
	if _, err := canonicalPath(filepath.Join(root, "missing")); err == nil {
		t.Fatal("不存在路径应被拒绝")
	}
	if _, err := canonicalPath("relative"); err == nil {
		t.Fatal("相对路径应被拒绝")
	}
}

func TestWithinRejectsParentEscape(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "workspace")
	for _, path := range []string{root, filepath.Join(root, "nested", "file.txt")} {
		if !within(root, path) {
			t.Fatalf("工作区内路径被拒绝: %s", path)
		}
	}
	for _, path := range []string{filepath.Join(root, "..", "outside"), filepath.Join(string(filepath.Separator), "tmp", "other")} {
		if within(root, path) {
			t.Fatalf("工作区外路径被接受: %s", path)
		}
	}
}
