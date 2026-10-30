package gate

import "bytes"

const redactedText = "[redacted]"

// secretRedactor 在分块流上抹掉已知密钥，保留可能跨块的前缀。
type secretRedactor struct {
	secrets [][]byte
	keep    int
	held    []byte
}

func newSecretRedactor(secrets []string) *secretRedactor {
	return newSecretRedactorOpts(secrets, false)
}

// newSecretRedactorOpts：errorBody 为真时额外抹掉密钥前 8 / 末 4 位片段（非 2xx 常见回显）。
func newSecretRedactorOpts(secrets []string, errorBody bool) *secretRedactor {
	r := &secretRedactor{}
	var frags [][]byte
	for _, s := range secrets {
		if len(s) < 4 {
			continue
		}
		b := []byte(s)
		if len(s) >= 8 {
			frags = append(frags, b)
			if errorBody {
				frags = append(frags, append([]byte(nil), b[:8]...))
				frags = append(frags, append([]byte(nil), b[len(b)-4:]...))
			}
		} else if errorBody {
			frags = append(frags, b)
		}
	}
	// 长优先，避免短片段先匹配
	for i := 0; i < len(frags); i++ {
		for j := i + 1; j < len(frags); j++ {
			if len(frags[j]) > len(frags[i]) {
				frags[i], frags[j] = frags[j], frags[i]
			}
		}
	}
	r.secrets = frags
	for _, frag := range frags {
		if n := len(frag) - 1; n > r.keep {
			r.keep = n
		}
	}
	return r
}

func (r *secretRedactor) scrub(in []byte) []byte {
	out := in
	mark := []byte(redactedText)
	for _, s := range r.secrets {
		out = bytes.ReplaceAll(out, s, mark)
	}
	return out
}

// Push 返回可以安全写出的前缀。
func (r *secretRedactor) Push(in []byte) []byte {
	if len(r.secrets) == 0 {
		return in
	}
	data := append(r.held, in...)
	r.held = nil
	data = r.scrub(data)
	if r.keep <= 0 || len(data) <= r.keep {
		r.held = append([]byte(nil), data...)
		return nil
	}
	r.held = append([]byte(nil), data[len(data)-r.keep:]...)
	return append([]byte(nil), data[:len(data)-r.keep]...)
}

// Flush 写出剩余缓冲。调用方应在流结束时调用。
func (r *secretRedactor) Flush() []byte {
	if len(r.held) == 0 {
		return nil
	}
	out := r.scrub(r.held)
	r.held = nil
	return out
}
