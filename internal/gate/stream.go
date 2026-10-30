package gate

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

var (
	errStreamIdle  = errors.New("gate: upstream idle timeout")
	errStreamTotal = errors.New("gate: upstream total timeout")
)

func (h *Harbor) streamTimeouts() (idle, total time.Duration) {
	idle, total = h.idleTimeout, h.totalTimeout
	if idle <= 0 {
		idle = 60 * time.Second
	}
	if total <= 0 {
		total = 10 * time.Minute
	}
	return idle, total
}

func (h *Harbor) upstreamContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	_, total := h.streamTimeouts()
	return context.WithTimeout(parent, total)
}

// deadlineReader 在空闲或总时限到达时关闭底层响应体。
type deadlineReader struct {
	r        io.ReadCloser
	idle     time.Duration
	deadline time.Time
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	if !d.deadline.IsZero() && !time.Now().Before(d.deadline) {
		_ = d.r.Close()
		return 0, errStreamTotal
	}
	wait := d.idle
	if !d.deadline.IsZero() {
		if remain := time.Until(d.deadline); remain < wait {
			wait = remain
		}
	}
	if wait <= 0 {
		_ = d.r.Close()
		return 0, errStreamTotal
	}
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := d.r.Read(p)
		ch <- result{n, err}
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case got := <-ch:
		return got.n, got.err
	case <-timer.C:
		_ = d.r.Close()
		if !d.deadline.IsZero() && !time.Now().Before(d.deadline) {
			return 0, errStreamTotal
		}
		return 0, errStreamIdle
	}
}

// relayResponse 把上游响应写给沙箱，并抹掉已知密钥。
// 首字节超时且尚未写出时返回错误，调用方可以改成 504。
func relayResponse(w http.ResponseWriter, resp *http.Response, secrets []string, idle, total time.Duration) error {
	return relayResponseScan(w, resp, secrets, idle, total, nil)
}

// relayResponseScan 同 relayResponse；scan 非空时旁路扫描明文块中的 usage（不改写出内容）。
func relayResponseScan(w http.ResponseWriter, resp *http.Response, secrets []string, idle, total time.Duration, scan *usageScanner) error {
	defer resp.Body.Close()
	reader := &deadlineReader{
		r:        resp.Body,
		idle:     idle,
		deadline: time.Now().Add(total),
	}
	errorBody := resp.StatusCode < 200 || resp.StatusCode >= 300
	red := newSecretRedactorOpts(secrets, errorBody)
	buf := make([]byte, 32*1024)
	n, err := reader.Read(buf)
	if n == 0 && err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	copyHeaders(w.Header(), resp.Header)
	scrubHeaders(w.Header(), secrets)
	w.WriteHeader(resp.StatusCode)
	if n > 0 {
		if scan != nil {
			scan.Push(buf[:n])
		}
		if err := writeAll(w, red.Push(buf[:n])); err != nil {
			return err
		}
	}
	for {
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		n, err = reader.Read(buf)
		if n > 0 {
			if scan != nil {
				scan.Push(buf[:n])
			}
			if werr := writeAll(w, red.Push(buf[:n])); werr != nil {
				return werr
			}
		}
	}
	if err := writeAll(w, red.Flush()); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func writeAll(w io.Writer, b []byte) error {
	if len(b) == 0 {
		return nil
	}
	_, err := w.Write(b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return err
}

func scrubHeaders(h http.Header, secrets []string) {
	for key, values := range h {
		for i, value := range values {
			red := newSecretRedactor(secrets)
			out := red.Push([]byte(value))
			out = append(out, red.Flush()...)
			h[key][i] = string(out)
		}
	}
}

type statusWriter struct {
	http.ResponseWriter
	wrote bool
}

func (s *statusWriter) WriteHeader(code int) {
	s.wrote = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func writeRelay(w http.ResponseWriter, resp *http.Response, secrets []string, idle, total time.Duration) error {
	return writeRelayScan(w, resp, secrets, idle, total, nil)
}

func writeRelayScan(w http.ResponseWriter, resp *http.Response, secrets []string, idle, total time.Duration, scan *usageScanner) error {
	sw := &statusWriter{ResponseWriter: w}
	err := relayResponseScan(sw, resp, secrets, idle, total, scan)
	if err != nil && !sw.wrote {
		status := streamTimeoutStatus(err)
		msg := "upstream unavailable"
		if status == http.StatusGatewayTimeout {
			msg = "upstream timeout"
		}
		http.Error(w, msg, status)
	}
	return err
}

func streamTimeoutStatus(err error) int {
	if errors.Is(err, errStreamIdle) || errors.Is(err, errStreamTotal) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}
