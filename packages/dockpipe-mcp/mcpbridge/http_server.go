package mcpbridge

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// HTTP defaults — conservative for a control-plane edge.
const defaultMaxBodyBytes = 1 << 20 // 1 MiB

// HTTPConfig configures the MCP JSON-RPC HTTP listener.
type HTTPConfig struct {
	ListenAddr string // e.g. ":8443" or "127.0.0.1:8443"

	TLSCertFile string
	TLSKeyFile  string

	// APIKey is required for every HTTP request when KeyTierFile is empty (Bearer or X-API-Key).
	APIKey string

	// KeyTierFile is optional JSON: [{"key":"secret","tier":"readonly"}, ...].
	// When set, APIKey is ignored; each key maps to its own MCP tier (per-key IAM).
	KeyTierFile string

	// AllowInsecurePlainHTTP allows HTTP without TLS. Only permitted when ListenAddr is loopback-only.
	AllowInsecurePlainHTTP bool

	Log *log.Logger
}

// ServeHTTP runs JSON-RPC MCP at POST / and POST /mcp until ctx is cancelled.
// TLS is required unless AllowInsecurePlainHTTP is set and the bind address is loopback-only.
func (s *Server) ServeHTTP(ctx context.Context, cfg HTTPConfig) error {
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return errors.New("mcpbridge: empty ListenAddr")
	}
	if cfg.Log == nil {
		cfg.Log = log.New(os.Stderr, "mcpd: ", log.LstdFlags|log.Lmicroseconds)
	}

	keyTierPath := strings.TrimSpace(cfg.KeyTierFile)
	var keyTiers []keyTierEntry
	if keyTierPath != "" {
		var err error
		keyTiers, err = loadHTTPKeyTierFile(keyTierPath)
		if err != nil {
			return fmt.Errorf("mcpbridge: %w", err)
		}
	}
	if keyTierPath == "" && strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New("mcpbridge: HTTP requires MCP_HTTP_API_KEY (-api-key) or MCP_HTTP_KEY_TIERS_FILE (-key-tiers-file)")
	}
	if keyTierPath != "" && strings.TrimSpace(cfg.APIKey) != "" {
		cfg.Log.Printf("warning: MCP_HTTP_KEY_TIERS_FILE is set; ignoring MCP_HTTP_API_KEY")
	}

	useTLS := strings.TrimSpace(cfg.TLSCertFile) != "" && strings.TrimSpace(cfg.TLSKeyFile) != ""
	if !useTLS {
		if !cfg.AllowInsecurePlainHTTP {
			return errors.New("mcpbridge: HTTPS requires TLS cert and key files (MCP_TLS_CERT_FILE / MCP_TLS_KEY_FILE), or set MCP_HTTP_INSECURE_LOOPBACK=1 for plain HTTP on 127.0.0.1 only")
		}
		if !isLoopbackBind(cfg.ListenAddr) {
			return errors.New("mcpbridge: insecure HTTP is only allowed when binding to loopback (127.0.0.1, ::1, or localhost:port)")
		}
		cfg.Log.Printf("warning: serving MCP over plain HTTP on loopback only — not for production")
	}

	logW := cfg.Log.Writer()
	mux := http.NewServeMux()
	h := s.jsonRPCHandler(logW)
	if len(keyTiers) > 0 {
		h = httpKeyTierGate(keyTiers, h)
	} else {
		h = apiKeyGate(cfg.APIKey, h)
	}
	mux.Handle("/mcp", h)
	mux.Handle("/", h)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       180 * time.Second,
		MaxHeaderBytes:    1 << 15,
		Handler:           mux,
	}
	if useTLS {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		}
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if useTLS {
			cfg.Log.Printf("mcp HTTPS listening on %s", cfg.ListenAddr)
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			cfg.Log.Printf("mcp HTTP (insecure) listening on %s", cfg.ListenAddr)
			err = srv.ListenAndServe()
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if err == nil {
			return nil
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// acceptsJSONContentType allows empty (clients omit), types containing "json", and application/json (+json suffixes).
func acceptsJSONContentType(ct string) bool {
	ct = strings.TrimSpace(ct)
	if ct == "" {
		return true
	}
	ct = strings.ToLower(ct)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if strings.Contains(ct, "json") {
		return true
	}
	return false
}

func isLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// ":8443" binds all interfaces — not loopback-only
		if strings.HasPrefix(addr, ":") {
			return false
		}
		host = addr
	}
	switch strings.TrimSpace(host) {
	case "127.0.0.1", "::1", "localhost", "ip6-localhost":
		return true
	default:
		return false
	}
}

func apiKeyGate(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := ExpectedAPIKeyFromRequest(r)
		if !ConstantTimeEqualString(got, expected) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp", error="invalid_token"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func httpKeyTierGate(entries []keyTierEntry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := ExpectedAPIKeyFromRequest(r)
		tier, ok := lookupKeyTier(entries, got)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp", error="invalid_token"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithMCPTier(r.Context(), tier)))
	})
}

func (s *Server) jsonRPCHandler(logW io.Writer) http.Handler {
	lg := log.New(logW, "mcpd: ", log.LstdFlags|log.Lmicroseconds)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); !acceptsJSONContentType(ct) {
			http.Error(w, `{"error":"unsupported media type"}`, http.StatusUnsupportedMediaType)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, defaultMaxBodyBytes))
		if err != nil {
			http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		resp := s.handleMessage(r.Context(), body, logW)
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			lg.Printf("encode response: %v", err)
		}
	})
}
