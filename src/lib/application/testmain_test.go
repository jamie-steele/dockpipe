package application

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Tests often run with cwd = the dockpipe git checkout; without this, mergeOpInjectFromProjectIfEnabled
	// would run op inject against the real dockpipe.config.json + .env.op.template (needs `op` CLI).
	if os.Getenv("DOCKPIPE_OP_INJECT") == "" {
		os.Setenv("DOCKPIPE_OP_INJECT", "0")
	}
	os.Exit(m.Run())
}
