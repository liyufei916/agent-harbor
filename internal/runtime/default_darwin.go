//go:build darwin

package runtime

// NewDefault 返回当前平台默认 Runtime（macOS Seatbelt）。
func NewDefault() Runtime {
	return NewMacOS()
}
