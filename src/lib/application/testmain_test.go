package application

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Tests often run with cwd = the dockpipe git checkout; without this, mergeOpInjectFromProjectIfEnabled
	// would run op inject against the real dockpipe.config.json + vault template (needs `op` CLI).
	if os.Getenv("DOCKPIPE_OP_INJECT") == "" {
		os.Setenv("DOCKPIPE_OP_INJECT", "0")
	}
	// Default run path compiles transitive deps for --workflow; tests use fake seams and must not hit compile.
	if os.Getenv("DOCKPIPE_COMPILE_DEPS") == "" {
		os.Setenv("DOCKPIPE_COMPILE_DEPS", "0")
	}
	os.Exit(m.Run())
}
