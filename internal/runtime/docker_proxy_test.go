package runtime

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestTCPProxyForwardsAllowedOnly(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	go func() {
		c, err := upstream.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.WriteString(c, "pong")
	}()

	portMap, proxies, err := startAllowedProxies([]string{upstream.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer closeProxies(proxies)
	_, upPort, _ := net.SplitHostPort(upstream.Addr().String())
	listenPort := portMap[upPort]
	if listenPort == "" {
		t.Fatal(portMap)
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+listenPort, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	buf := make([]byte, 4)
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := io.ReadFull(c, buf)
	if err != nil || string(buf[:n]) != "pong" {
		t.Fatalf("got %q err=%v", buf[:n], err)
	}
}

func TestRewriteGateHostMapped(t *testing.T) {
	got := rewriteGateHostMapped("http://127.0.0.1:18080/v1", map[string]string{"18080": "9"})
	if got != "http://host.docker.internal:9/v1" {
		t.Fatal(got)
	}
}
