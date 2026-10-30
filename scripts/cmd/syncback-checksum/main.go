package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liyufei916/agent-harbor/internal/runtime"
)

func hashFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "MISSING"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: syncback-checksum <cloneDir> <sourceDir>")
		os.Exit(2)
	}
	clone, src := os.Args[1], os.Args[2]
	beforeExtra := filepath.Join(src, "ONLY_SRC_KEEP.txt")
	_ = os.WriteFile(beforeExtra, []byte("keep-source-extra\n"), 0600)
	demo := filepath.Join(clone, "SYNCBACK_DEMO.md")
	_ = os.WriteFile(demo, []byte("harbor-syncback-ok\n"), 0600)
	beforeDemo := hashFile(filepath.Join(src, "SYNCBACK_DEMO.md"))
	beforeExtraH := hashFile(beforeExtra)
	if err := runtime.SyncWorkspace(context.Background(), clone, src); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	afterDemo := hashFile(filepath.Join(src, "SYNCBACK_DEMO.md"))
	afterExtra := hashFile(beforeExtra)
	fmt.Printf("SYNCBACK_DEMO.md before=%s after=%s\n", beforeDemo, afterDemo)
	fmt.Printf("ONLY_SRC_KEEP.txt before=%s after=%s (must equal)\n", beforeExtraH, afterExtra)
	body, _ := os.ReadFile(filepath.Join(src, "SYNCBACK_DEMO.md"))
	fmt.Printf("body=%q\n", string(body))
	want := hashFile(demo)
	if afterExtra != beforeExtraH || afterDemo != want {
		fmt.Fprintln(os.Stderr, "checksum mismatch")
		os.Exit(1)
	}
	fmt.Println("OK: overwrite same-name + keep extra")
}
