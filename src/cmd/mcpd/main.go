// Command mcpd is a thin MCP (Model Context Protocol) bridge over DockPipe/DorkPipe.
// See docs/mcp-architecture.md and src/lib/mcpbridge/README.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"dockpipe/src/lib/mcpbridge"
)

// Version is set at link time (see Makefile).
var Version = "dev"

func main() {
	if v := strings.TrimSpace(Version); v != "" && v != "dev" {
		_ = os.Setenv("DOCKPIPE_MCP_SERVER_VERSION", v)
	}

	httpListen := flag.String("http", "", "listen address for MCP over HTTPS (e.g. :8443); env MCP_HTTP_LISTEN; empty = stdio")
	apiKey := flag.String("api-key", "", "HTTP auth when no key-tiers-file; env MCP_HTTP_API_KEY")
	keyTiersFile := flag.String("key-tiers-file", "", "JSON map of API keys to tiers (replaces -api-key); env MCP_HTTP_KEY_TIERS_FILE")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file; env MCP_TLS_CERT_FILE")
	tlsKey := flag.String("tls-key", "", "TLS private key file; env MCP_TLS_KEY_FILE")
	insecureLoopback := flag.Bool("insecure-loopback", false, "plain HTTP on loopback only; env MCP_HTTP_INSECURE_LOOPBACK=1")
	mcpTier := flag.String("mcp-tier", "", "MCP IAM tier: readonly|validate|exec (sets DOCKPIPE_MCP_TIER)")
	flag.Parse()

	if v := strings.TrimSpace(*mcpTier); v != "" {
		_ = os.Setenv("DOCKPIPE_MCP_TIER", v)
	}

	listen := strings.TrimSpace(*httpListen)
	if listen == "" {
		listen = strings.TrimSpace(os.Getenv("MCP_HTTP_LISTEN"))
	}

	srv := mcpbridge.NewServer(Version)

	if listen != "" {
		kt := strings.TrimSpace(*keyTiersFile)
		if kt == "" {
			kt = strings.TrimSpace(os.Getenv("MCP_HTTP_KEY_TIERS_FILE"))
		}
		key := strings.TrimSpace(*apiKey)
		if key == "" {
			key = strings.TrimSpace(os.Getenv("MCP_HTTP_API_KEY"))
		}
		cert := strings.TrimSpace(*tlsCert)
		if cert == "" {
			cert = strings.TrimSpace(os.Getenv("MCP_TLS_CERT_FILE"))
		}
		keyFile := strings.TrimSpace(*tlsKey)
		if keyFile == "" {
			keyFile = strings.TrimSpace(os.Getenv("MCP_TLS_KEY_FILE"))
		}
		allowInsecure := *insecureLoopback || strings.TrimSpace(os.Getenv("MCP_HTTP_INSECURE_LOOPBACK")) == "1"

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		cfg := mcpbridge.HTTPConfig{
			ListenAddr:             listen,
			TLSCertFile:            cert,
			TLSKeyFile:             keyFile,
			APIKey:                 key,
			KeyTierFile:            kt,
			AllowInsecurePlainHTTP: allowInsecure,
		}
		if err := srv.ServeHTTP(ctx, cfg); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Do not write to stderr in stdio mode unless explicitly enabled: many MCP hosts
	// surface stderr as [error], which looks like a failed handshake when this is only a status line.
	if strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_DEBUG")) != "" {
		fmt.Fprintf(os.Stderr, "[dockpipe-mcpd] stdio mode (version %s); awaiting MCP JSON-RPC on stdin\n", srv.Version)
	}
	if err := srv.ServeStdio(os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}
