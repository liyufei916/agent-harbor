//go:build darwin

package secrets

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = "com.liyufei916.agent-harbor"

type keychainVault struct{}

// unavailableVault：Darwin 上 Keychain 不可用时拒绝落明文文件（禁止静默回退）。
type unavailableVault struct {
	reason string
}

func (u unavailableVault) Set(string, []byte) error {
	return fmt.Errorf("%s", u.reason)
}
func (u unavailableVault) Get(string) ([]byte, error) {
	return nil, fmt.Errorf("%s", u.reason)
}
func (u unavailableVault) Delete(string) error {
	return fmt.Errorf("%s", u.reason)
}

// vaultNotice 供 UI/诊断读取；空=钥匙串正常。
var vaultNotice string

// VaultBackendNotice 返回凭据介质状态说明；空表示 Keychain 可用。
func VaultBackendNotice() string {
	return vaultNotice
}

func newPlatformVault(dir string) vault {
	_ = dir // 保留签名与非 Darwin 一致；Darwin 不回退文件库
	v := keychainVault{}
	probeID := "__harbor_vault_probe__"
	if err := v.Set(probeID, []byte("ok")); err != nil {
		msg := "macOS Keychain 不可用，已拒绝回退到明文文件库。请解锁登录钥匙串并授予 Agent Harbor 访问权限后重启。"
		vaultNotice = msg
		return unavailableVault{reason: msg}
	}
	_ = v.Delete(probeID)
	vaultNotice = ""
	return v
}

func validateVaultID(id string) error {
	if id == "" || strings.ContainsAny(id, "\n\r\x00") {
		return fmt.Errorf("invalid vault id")
	}
	return nil
}

func (keychainVault) Set(id string, secret []byte) error {
	if err := validateVaultID(id); err != nil {
		return err
	}
	_ = keychainVault{}.Delete(id)
	// 避免 -w <明文> 进入 argv：经 security -i + -X hex，口令只走 stdin。
	script := fmt.Sprintf(
		"add-generic-password -s %s -a %s -U -X %s\n",
		shellSingleQuote(keychainService),
		shellSingleQuote(id),
		hex.EncodeToString(secret),
	)
	cmd := exec.Command("/usr/bin/security", "-i")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain set: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (keychainVault) Get(id string) ([]byte, error) {
	if err := validateVaultID(id); err != nil {
		return nil, err
	}
	cmd := exec.Command("/usr/bin/security", "find-generic-password",
		"-s", keychainService,
		"-a", id,
		"-w",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrNotFound
	}
	return bytes.TrimSpace(out), nil
}

func (keychainVault) Delete(id string) error {
	if err := validateVaultID(id); err != nil {
		return err
	}
	cmd := exec.Command("/usr/bin/security", "delete-generic-password",
		"-s", keychainService,
		"-a", id,
	)
	_ = cmd.Run() // 不存在也当成功
	return nil
}
