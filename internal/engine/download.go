package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// fetchFunc downloads url into w; injectable for tests.
type fetchFunc func(ctx context.Context, url string, w io.Writer) error

func defaultFetch(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "agent-harbor-engine/1")
	// GitHub release assets can be slow on some networks; allow long transfer.
	client := &http.Client{Timeout: 20 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", res.StatusCode)
	}
	_, err = io.Copy(w, res.Body)
	return err
}

func autoDownloadEnabled() bool {
	v := strings.TrimSpace(os.Getenv("HARBOR_ENGINE_AUTO_DOWNLOAD"))
	if v == "0" || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") {
		return false
	}
	return true
}

// ensurePinnedBins downloads missing required binaries into binDir when auto-download is on.
// Download phase maps to overall progress 3–75%.
func (c *Colima) ensurePinnedBins(ctx context.Context) error {
	arts := pinnedArtifacts()
	if len(arts) == 0 {
		return errEngine("当前平台不支持钉扎引擎下载（仅 darwin/arm64 与 darwin/amd64）。")
	}
	fetch := c.fetch
	if fetch == nil {
		fetch = defaultFetch
	}
	var pending []pinnedArtifact
	var totalBytes int64
	for _, a := range arts {
		dest := filepath.Join(c.binDir(), a.Name)
		if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
			continue
		}
		pending = append(pending, a)
		n := a.ExpectedBytes
		if n <= 0 {
			n = 10_000_000
		}
		totalBytes += n
	}
	if len(pending) == 0 {
		return nil
	}
	if !autoDownloadEnabled() {
		return errEngine("缺少 " + pending[0].Name + "。可开启自动下载或手动放入引擎 bin 目录。")
	}
	const dlLo, dlHi = 3, 75
	var doneBytes int64
	for _, a := range pending {
		dest := filepath.Join(c.binDir(), a.Name)
		exp := a.ExpectedBytes
		if exp <= 0 {
			exp = 10_000_000
		}
		baseDone := doneBytes
		onByte := func(n int64) {
			cur := baseDone + n
			if cur > totalBytes {
				cur = totalBytes
			}
			frac := float64(cur) / float64(totalBytes)
			pct := dlLo + int(frac*float64(dlHi-dlLo))
			c.report(pct, "下载 "+a.Name+"… "+strconv.Itoa(clampPercent(pct))+"%")
		}
		c.report(dlLo+int(float64(baseDone)/float64(totalBytes)*float64(dlHi-dlLo)), "下载 "+a.Name+"…")
		if err := c.installArtifact(ctx, fetch, a, dest, onByte); err != nil {
			return err
		}
		doneBytes += exp
	}
	return nil
}

type progressWriter struct {
	w    io.Writer
	n    int64
	last int64
	on   func(int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.n += int64(n)
	if p.on != nil && (p.n-p.last >= 256*1024 || err != nil) {
		p.last = p.n
		p.on(p.n)
	}
	return n, err
}

func (c *Colima) installArtifact(ctx context.Context, fetch fetchFunc, a pinnedArtifact, dest string, onByte func(int64)) error {
	tmpDir, err := os.MkdirTemp(c.root(), "dl-*")
	if err != nil {
		return errEngine("无法创建下载临时目录。")
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	rawPath := filepath.Join(tmpDir, "payload")
	f, err := os.OpenFile(rawPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errEngine("无法写入下载文件。")
	}
	hash := sha256.New()
	w := io.Writer(io.MultiWriter(f, hash))
	if onByte != nil {
		w = &progressWriter{w: w, on: onByte}
	}
	if err := fetch(ctx, a.URL, w); err != nil {
		_ = f.Close()
		detail := strings.TrimSpace(err.Error())
		if detail == "" {
			detail = "网络错误"
		}
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(detail, "context deadline") {
			detail = "超时（GitHub 较慢时可重试，或手动放入引擎 bin）"
		}
		return errEngine("下载引擎组件失败：" + a.Name + "（" + detail + "）。")
	}
	if onByte != nil {
		onByte(a.ExpectedBytes)
	}
	if err := f.Close(); err != nil {
		return errEngine("无法关闭下载文件。")
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(sum, a.SHA256) {
		return errEngine("引擎组件校验失败：" + a.Name + "。")
	}

	switch a.Kind {
	case artifactRaw:
		if err := os.Rename(rawPath, dest); err != nil {
			data, rerr := os.ReadFile(rawPath)
			if rerr != nil {
				return errEngine("无法安装 " + a.Name + "。")
			}
			if err := os.WriteFile(dest, data, 0755); err != nil {
				return errEngine("无法安装 " + a.Name + "。")
			}
		} else if err := os.Chmod(dest, 0755); err != nil {
			return errEngine("无法设置 " + a.Name + " 可执行权限。")
		}
	case artifactTarGz:
		if a.ExtractRoot != "" {
			prefix := filepath.Join(c.root(), a.ExtractRoot)
			_ = os.RemoveAll(prefix)
			if err := os.MkdirAll(prefix, 0700); err != nil {
				return errEngine("无法创建 " + a.Name + " 目录。")
			}
			if err := extractTarGzAll(rawPath, prefix); err != nil {
				return errEngine("无法解压 " + a.Name + "。")
			}
			src := filepath.Join(prefix, "bin", a.Name)
			if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
				return errEngine("无法安装 " + a.Name + "。")
			}
			if err := os.Symlink(src, dest); err != nil {
				// fallback copy
				data, rerr := os.ReadFile(src)
				if rerr != nil {
					return errEngine("无法安装 " + a.Name + "。")
				}
				if err := os.WriteFile(dest, data, 0755); err != nil {
					return errEngine("无法安装 " + a.Name + "。")
				}
			}
			break
		}
		if err := extractTarGzMember(rawPath, a.Member, dest); err != nil {
			return errEngine("无法解压 " + a.Name + "。")
		}
		if err := os.Chmod(dest, 0755); err != nil {
			return errEngine("无法设置 " + a.Name + " 可执行权限。")
		}
	default:
		return errEngine("未知引擎组件格式。")
	}
	return nil
}

func extractTarGzAll(archive, destDir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(destDir, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := hdr.FileInfo().Mode()
			if mode == 0 {
				mode = 0644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			_ = os.Remove(target)
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			_ = os.Symlink(hdr.Linkname, target)
		}
	}
}

func extractTarGzMember(archive, member, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	member = strings.TrimPrefix(member, "./")
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("member %s not found", member)
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name != member && filepath.Base(name) != filepath.Base(member) {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
}
