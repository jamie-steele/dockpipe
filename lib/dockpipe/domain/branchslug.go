package domain

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
)

// branchSlugAdjectives and branchSlugNouns are mixed dev/infra-flavored tokens.
// Four picks (adj-noun-adj-noun) give |A|×|N|×|A|×|N| combinations — on the order of 10^8+.
// Keep scripts/branch-slug.sh (sourced by dockpipe-legacy.sh) in sync when editing these lists.
var branchSlugAdjectives = []string{
	"able", "apt", "azure", "bold", "brisk", "bright", "calm", "clear", "cool", "crisp",
	"curious", "dapper", "eager", "fancy", "fast", "fleet", "fresh", "gentle", "grand", "happy",
	"honest", "jolly", "keen", "kind", "light", "lively", "lucky", "merry", "mighty", "mint",
	"modern", "narrow", "neat", "nimble", "noble", "open", "patient", "polite", "proud", "quick",
	"quiet", "rapid", "rare", "ready", "real", "rich", "robust", "round", "sharp", "silent",
	"simple", "sleek", "smart", "smooth", "social", "solid", "sound", "spry", "steady", "still",
	"stoic", "strong", "subtle", "super", "sweet", "swift", "tidy", "tiny", "true", "vivid",
	"warm", "wild", "wise", "witty", "young", "zesty",
}

var branchSlugNouns = []string{
	"adapter", "anchor", "api", "array", "atom", "badge", "beacon", "binary", "bitmap", "block",
	"branch", "bridge", "buffer", "bundle", "byte", "cache", "canvas", "channel", "chunk", "cipher",
	"client", "cloud", "cluster", "commit", "cookie", "core", "cron", "daemon", "delta", "deploy",
	"digest", "docker", "driver", "edge", "engine", "event", "fiber", "field", "filter", "frame",
	"gateway", "graph", "grid", "handler", "hash", "header", "heap", "hook", "http", "index",
	"ingress", "kernel", "lambda", "layer", "ledger", "lint", "loader", "lock", "log", "loop",
	"matrix", "merge", "metric", "mirror", "module", "mount", "mutex", "ngrok", "node", "packet",
	"patch", "peer", "pipe", "pixel", "pod", "poll", "portal", "probe", "proxy", "pulse",
	"queue", "quota", "raft", "range", "replica", "repo", "request", "ring", "route", "runner",
	"schema", "scope", "script", "server", "session", "shard", "shell", "signal", "socket", "spark",
	"stack", "stage", "stream", "subnet", "switch", "sync", "table", "token", "trace", "tunnel",
	"vector", "vertex", "volume", "voucher", "watch", "webhook", "widget", "worker", "worktree", "zone",
}

func rndIndex(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("invalid max %d", max)
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	u := binary.BigEndian.Uint64(buf[:])
	return int(u % uint64(max)), nil
}

// RandomWorkBranchSlug returns a lowercase hyphenated slug safe for git branch names,
// e.g. "steady-ngrok-calm-worktree" (two adjective–noun pairs, crypto-random).
func RandomWorkBranchSlug() (string, error) {
	ia, err := rndIndex(len(branchSlugAdjectives))
	if err != nil {
		return "", err
	}
	na, err := rndIndex(len(branchSlugNouns))
	if err != nil {
		return "", err
	}
	ib, err := rndIndex(len(branchSlugAdjectives))
	if err != nil {
		return "", err
	}
	nb, err := rndIndex(len(branchSlugNouns))
	if err != nil {
		return "", err
	}
	parts := []string{
		branchSlugAdjectives[ia],
		branchSlugNouns[na],
		branchSlugAdjectives[ib],
		branchSlugNouns[nb],
	}
	return strings.Join(parts, "-"), nil
}
