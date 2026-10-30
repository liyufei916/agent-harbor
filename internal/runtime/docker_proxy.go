package runtime

import (
	"io"
	"net"
	"strings"
	"sync"
)

// tcpProxy forwards 127.0.0.1:listen → dial (one AllowedTCP target).
// Guest uses host.docker.internal:listenPort. On Docker Desktop, guests may still
// dial other host loopback ports directly; see P2-DOCKER residual note.
type tcpProxy struct {
	ln   net.Listener
	dial string
	done chan struct{}
	once sync.Once
}

func startTCPProxy(dialAddr string) (*tcpProxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &tcpProxy{ln: ln, dial: dialAddr, done: make(chan struct{})}
	go p.serve()
	return p, nil
}

func (p *tcpProxy) listenPort() string {
	_, port, err := net.SplitHostPort(p.ln.Addr().String())
	if err != nil {
		return ""
	}
	return port
}

func (p *tcpProxy) serve() {
	for {
		conn, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.relay(conn)
	}
}

func (p *tcpProxy) relay(c net.Conn) {
	defer c.Close()
	up, err := net.Dial("tcp", p.dial)
	if err != nil {
		return
	}
	defer up.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(up, c) }()
	go func() { defer wg.Done(); _, _ = io.Copy(c, up) }()
	wg.Wait()
}

func (p *tcpProxy) Close() {
	p.once.Do(func() {
		close(p.done)
		_ = p.ln.Close()
	})
}

func startAllowedProxies(allowedTCP []string) (map[string]string, []*tcpProxy, error) {
	portMap := make(map[string]string, len(allowedTCP))
	proxies := make([]*tcpProxy, 0, len(allowedTCP))
	fail := func(err error) (map[string]string, []*tcpProxy, error) {
		closeProxies(proxies)
		return nil, nil, err
	}
	for _, addr := range allowedTCP {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || host != "127.0.0.1" || port == "" {
			return fail(runtimeError("INVALID_ARGUMENT"))
		}
		p, err := startTCPProxy(addr)
		if err != nil {
			return fail(runtimeError("ISOLATION_UNAVAILABLE"))
		}
		listen := p.listenPort()
		if listen == "" {
			p.Close()
			return fail(runtimeError("ISOLATION_UNAVAILABLE"))
		}
		portMap[port] = listen
		proxies = append(proxies, p)
	}
	return portMap, proxies, nil
}

func closeProxies(proxies []*tcpProxy) {
	for _, p := range proxies {
		if p != nil {
			p.Close()
		}
	}
}

// rewriteGateHostMapped: 127.0.0.1:gatePort → host.docker.internal:proxyPort.
func rewriteGateHostMapped(value string, gateToProxy map[string]string) string {
	const loop = "127.0.0.1:"
	for {
		i := strings.Index(value, loop)
		if i < 0 {
			return value
		}
		rest := value[i+len(loop):]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		gatePort := rest[:j]
		proxyPort, ok := gateToProxy[gatePort]
		if !ok || proxyPort == "" {
			return value
		}
		value = value[:i] + dockerGateHost + ":" + proxyPort + rest[j:]
	}
}
