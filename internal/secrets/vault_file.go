package secrets

import (
	"os"
	"path/filepath"
)

// fileVault：密钥写在 dir 下 0600 文件。仅非 Darwin 默认；Darwin 禁止静默回退到本库。
type fileVault struct {
	dir string
}

func newFileVault(dir string) vault {
	_ = os.MkdirAll(dir, 0700)
	return fileVault{dir: dir}
}

func (v fileVault) path(id string) string {
	return filepath.Join(v.dir, id+".key")
}

func (v fileVault) Set(id string, secret []byte) error {
	return os.WriteFile(v.path(id), secret, 0600)
}

func (v fileVault) Get(id string) ([]byte, error) {
	b, err := os.ReadFile(v.path(id))
	if err != nil {
		return nil, ErrNotFound
	}
	return b, nil
}

func (v fileVault) Delete(id string) error {
	_ = os.Remove(v.path(id))
	return nil
}
