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

func TestDSHEnvironmentCannotEscapePrivateHome(t *testing.T) {
	if _, err := launchEnvironment("/private/home", "", map[string]string{"DSH_HOME": "/Users/host/.dsh"}); err == nil {
		t.Fatal("允许借用宿主 HOME")
	}
	if _, err := launchEnvironment("/private/home", "", map[string]string{"DSH_TELEMETRY_DISABLED": ""}); err == nil {
		t.Fatal("允许关闭遥测禁用约束")
	}
}
