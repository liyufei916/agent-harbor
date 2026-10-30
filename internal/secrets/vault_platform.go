//go:build !darwin

package secrets

func newPlatformVault(dir string) vault {
	return newFileVault(dir)
}

// VaultBackendNotice 非 Darwin 使用文件库；产品基线为 macOS，此处仅占位。
func VaultBackendNotice() string {
	return "当前平台使用本机文件库保存密钥（非 macOS Keychain）。"
}
