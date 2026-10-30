package gate

import (
	"regexp"
	"strconv"
)

// usageScanner 在流式转发旁路扫描模型 usage，不缓冲整包、不改写出字节。
type usageScanner struct {
	pending []byte
	input   int64
	output  int64
}

var (
	reInputTokens  = regexp.MustCompile(`(?i)"(?:input_tokens|prompt_tokens)"\s*:\s*(\d+)`)
	reOutputTokens = regexp.MustCompile(`(?i)"(?:output_tokens|completion_tokens)"\s*:\s*(\d+)`)
)

const usageScanCap = 96 << 10

func (u *usageScanner) Push(p []byte) {
	if u == nil || len(p) == 0 {
		return
	}
	u.pending = append(u.pending, p...)
	if len(u.pending) > usageScanCap {
		u.pending = u.pending[len(u.pending)-usageScanCap:]
	}
	u.harvest()
}

func (u *usageScanner) harvest() {
	if len(u.pending) == 0 {
		return
	}
	s := string(u.pending)
	if m := reInputTokens.FindAllStringSubmatch(s, -1); len(m) > 0 {
		if n, err := strconv.ParseInt(m[len(m)-1][1], 10, 64); err == nil && n >= 0 {
			u.input = n
		}
	}
	if m := reOutputTokens.FindAllStringSubmatch(s, -1); len(m) > 0 {
		if n, err := strconv.ParseInt(m[len(m)-1][1], 10, 64); err == nil && n >= 0 {
			u.output = n
		}
	}
}

func (u *usageScanner) Totals() (input, output int64) {
	if u == nil {
		return 0, 0
	}
	u.harvest()
	return u.input, u.output
}
