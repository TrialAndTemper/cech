package evalrun

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The ports inside a batch's internal network: the proxy's, and the one
// on which the forwarder takes telemetry to the receiver's socket.
const (
	proxyPort = 3128
	otlpPort  = 4318
	// proxyHost and otlpHost are the names of the proxy and the forwarder
	// on the internal network.
	proxyHost = "proxy"
	otlpHost  = "otlp"
	// ctrOTLP is where the forwarder's container mounts the folder of the
	// receiver's socket, and OTLPSocket the socket's name in it.
	ctrOTLP    = "/cech/otlp"
	OTLPSocket = "otlp.sock"
)

// Batch is the network that one harness's runs share in one cech eval:
// an internal network that reaches nothing and two containers on it,
// started from the eval image: a proxy that lets through only the
// harness's hosts, and a forwarder that takes telemetry to the
// receiver's Unix socket on the host, so nothing listens on a network
// port of the host (criterion 15).
type Batch struct {
	e        *Engine
	s        Settings
	harness  string
	id       string
	internal string
	outside  string
	proxy    string
	otlp     string
	// Tools is the folder with Claude Code that InstallClaudeCode
	// filled, mounted read-only into each Claude Code run, or "".
	Tools string
}

// StartBatch creates the batch's networks and starts its proxy and its
// forwarder, which takes telemetry to the receiver listening on the
// socket OTLPSocket in the folder otlpDir. scratch is a folder for the proxy's settings,
// which Close leaves for the caller to remove.
func StartBatch(ctx context.Context, e *Engine, s Settings, harness, otlpDir, scratch string) (*Batch, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	b := &Batch{e: e, s: s, harness: harness, id: id}
	b.internal = "cech-eval-" + id
	b.outside = "cech-eval-" + id + "-out"
	b.proxy = "cech-eval-" + id + "-proxy"
	b.otlp = "cech-eval-" + id + "-otlp"
	conf := filepath.Join(scratch, "proxy")
	if err := os.MkdirAll(conf, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(conf, "tinyproxy.conf"), []byte(tinyproxyConf), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(conf, "filter"), []byte(Filter(s.Hosts[harness])), 0o600); err != nil {
		return nil, err
	}
	steps := [][]string{
		{"network", "create", "--internal", b.internal},
		{"network", "create", b.outside},
		append(append([]string{
			"run", "--detach", "--name", b.proxy, "--network", b.internal, "--network-alias", proxyHost, "--read-only",
		}, append(e.confined(id), e.user()...)...),
			e.mount(conf, "/cech/proxy", true), s.Image,
			"tinyproxy", "-d", "-c", "/cech/proxy/tinyproxy.conf"),
		{"network", "connect", b.outside, b.proxy},
		// SELinux lets no confined container connect to a socket a host
		// process listens on, so the forwarder runs without its label. It
		// runs socat alone, on the internal network alone, with no
		// capabilities and a read-only root; the proxy, which reaches
		// outside, keeps its label.
		append(append([]string{
			"run", "--detach", "--name", b.otlp, "--network", b.internal, "--network-alias", otlpHost,
			"--security-opt=label=disable", "--read-only",
		}, append(e.confined(id), e.user()...)...),
			e.plain(otlpDir, ctrOTLP), s.Image,
			"socat", fmt.Sprintf("TCP-LISTEN:%d,fork,reuseaddr", otlpPort), "UNIX-CONNECT:"+ctrOTLP+"/"+OTLPSocket),
	}
	for _, args := range steps {
		if _, _, err := e.Run(ctx, nil, args...); err != nil {
			b.Close(context.WithoutCancel(ctx))
			return nil, err
		}
	}
	return b, nil
}

// Close removes every container of the batch, then the networks,
// ignoring what is already gone, so an interrupted cech eval leaves no
// container behind (criterion 30).
func (b *Batch) Close(ctx context.Context) {
	b.e.RemoveBatch(ctx, b.id)
	_, _, _ = b.e.Run(ctx, nil, "network", "rm", "--force", b.outside)
	_, _, _ = b.e.Run(ctx, nil, "network", "rm", "--force", b.internal)
}

// tinyproxyConf lets a run reach only the hosts of the filter file, on
// port 443, and lists no upstream: anything else is refused.
const tinyproxyConf = `Port 3128
Listen 0.0.0.0
Timeout 600
MaxClients 64
LogLevel Warning
FilterDefaultDeny Yes
FilterType ere
FilterURLs Off
Filter "/cech/proxy/filter"
ConnectPort 443
`

// Filter is the proxy's filter file for hosts: one extended regular
// expression per line, each matching exactly one host name.
func Filter(hosts []string) string {
	var b strings.Builder
	for _, h := range hosts {
		b.WriteString("^" + regexp.QuoteMeta(h) + "$\n")
	}
	return b.String()
}

func randomID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
