package runtime

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// SyncWorkspace 将 source 目录内容合并写入 destination（覆盖同名常规文件，不删 destination 多余文件）。
// 限制与 cloneWorkspace 相同：拒符号链接/特殊文件；单文件 ≤64MiB；总量 ≤1GiB；节点 ≤10 万。
func SyncWorkspace(ctx context.Context, source, destination string) error {
	if source == "" || destination == "" {
		return runtimeError("INVALID_ARGUMENT")
	}
	srcAbs, err := filepath.Abs(source)
	if err != nil {
		return runtimeError("WORKSPACE_DENIED")
	}
	dstAbs, err := filepath.Abs(destination)
	if err != nil {
		return runtimeError("WORKSPACE_DENIED")
	}
	if resolved, err := filepath.EvalSymlinks(srcAbs); err == nil {
		srcAbs = resolved
	} else {
		return runtimeError("WORKSPACE_DENIED")
	}
	if resolved, err := filepath.EvalSymlinks(dstAbs); err == nil {
		dstAbs = resolved
	} else if !os.IsNotExist(err) {
		return runtimeError("WORKSPACE_DENIED")
	} else {
		dstAbs = filepath.Clean(dstAbs)
	}
	if srcAbs == dstAbs {
		return runtimeError("INVALID_ARGUMENT")
	}
	if nestedPath(srcAbs, dstAbs) || nestedPath(dstAbs, srcAbs) {
		return runtimeError("INVALID_ARGUMENT")
	}
	info, err := os.Stat(srcAbs)
	if err != nil || !info.IsDir() {
		return runtimeError("WORKSPACE_DENIED")
	}
	if err := os.MkdirAll(dstAbs, 0700); err != nil {
		return runtimeError("WORKSPACE_DENIED")
	}
	root, err := os.OpenRoot(srcAbs)
	if err != nil {
		return runtimeError("WORKSPACE_DENIED")
	}
	defer root.Close()
	remaining := int64(1 << 30)
	count := 0
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return runtimeError("CANCELLED")
		}
		if walkErr != nil {
			return runtimeError("WORKSPACE_DENIED")
		}
		if path == "." {
			return nil
		}
		count++
		if count > 100000 {
			return runtimeError("WORKSPACE_DENIED")
		}
		// 导回跳过符号链接/特殊文件（如 Claude 写入的 .claude/debug/latest），
		// 不中断整次 SyncBack；克隆入沙箱仍拒绝链接（见 clone.go）。
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return runtimeError("WORKSPACE_DENIED")
		}
		target := filepath.Join(dstAbs, filepath.FromSlash(path))
		if info.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > 64<<20 {
			return runtimeError("WORKSPACE_DENIED")
		}
		in, err := root.Open(path)
		if err != nil {
			return runtimeError("WORKSPACE_DENIED")
		}
		defer in.Close()
		opened, err := in.Stat()
		if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
			return runtimeError("WORKSPACE_DENIED")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return runtimeError("WORKSPACE_DENIED")
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600|info.Mode().Perm()&0100)
		if err != nil {
			return runtimeError("WORKSPACE_DENIED")
		}
		defer out.Close()
		limit := min(int64(64<<20), remaining)
		buffer := make([]byte, 32<<10)
		var copied int64
		for {
			if ctx.Err() != nil {
				return runtimeError("CANCELLED")
			}
			n, readErr := in.Read(buffer)
			copied += int64(n)
			if copied > limit {
				return runtimeError("WORKSPACE_DENIED")
			}
			if _, err := out.Write(buffer[:n]); err != nil {
				return runtimeError("WORKSPACE_DENIED")
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return runtimeError("WORKSPACE_DENIED")
			}
		}
		remaining -= copied
		if err := out.Close(); err != nil {
			return runtimeError("WORKSPACE_DENIED")
		}
		return nil
	})
}

// nestedPath 为 true 表示 child 位于 parent 之内（含自身已在外层排除）。
func nestedPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == ".." || rel == "." {
		return false
	}
	return filepath.IsLocal(rel)
}
