package application

import "testing"

func TestNormalizeTerraformCommandList(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plan", "plan"},
		{" init , plan ", "init,plan"},
		{"init plan", "init,plan"},
		{"init, plan , apply", "init,plan,apply"},
	}
	for _, tc := range tests {
		got := normalizeTerraformCommandList(tc.in)
		if got != tc.want {
			t.Errorf("normalizeTerraformCommandList(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidateTerraformCommands(t *testing.T) {
	if err := validateTerraformCommands("plan"); err != nil {
		t.Fatal(err)
	}
	if err := validateTerraformCommands("init plan"); err != nil {
		t.Fatal(err)
	}
	if err := validateTerraformCommands("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestJoinTerraformCommands(t *testing.T) {
	if got := joinTerraformCommands([]string{"init", "plan"}); got != "init,plan" {
		t.Fatalf("got %q", got)
	}
	if got := joinTerraformCommands([]string{"init, plan"}); got != "init,plan" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyTerraformCLIFromOpts(t *testing.T) {
	o, err := applyTerraformCLIFromOpts(&CliOpts{TfCommands: "plan", TfDryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if o["DOCKPIPE_TF_COMMANDS"] != "plan" || o["DOCKPIPE_TF_DRY_RUN"] != "1" {
		t.Fatalf("%#v", o)
	}
}
