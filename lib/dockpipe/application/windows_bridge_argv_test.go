package application

import (
	"reflect"
	"testing"
)

func TestSplitDockerMountHostContainer(t *testing.T) {
	cases := []struct {
		val, wantHost, wantCont string
	}{
		{"C:/Users/me:/work", "C:/Users/me", "/work"},
		{"namedvol:/path", "namedvol", "/path"},
		{"D:\\data:/mnt/data", `D:\data`, "/mnt/data"},
		{"./host:/c", "./host", "/c"},
		{"../up:/dest", "../up", "/dest"},
		{"nocolon", "nocolon", ""},
	}
	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			h, c := splitDockerMountHostContainer(tc.val)
			if h != tc.wantHost || c != tc.wantCont {
				t.Fatalf("split(%q) = (%q,%q) want (%q,%q)", tc.val, h, c, tc.wantHost, tc.wantCont)
			}
		})
	}
}

func TestTranslateMountSpec_fallbackDrive(t *testing.T) {
	got := translateMountSpec("Ubuntu", `C:\proj:/work:ro`)
	want := "/mnt/c/proj:/work:ro"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTranslateMountSpec_namedVolumeUnchanged(t *testing.T) {
	got := translateMountSpec("Ubuntu", "mydata:/var/lib/data:ro")
	want := "mydata:/var/lib/data:ro"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTranslateMountSpec_zSuffix(t *testing.T) {
	got := translateMountSpec("Ubuntu", `C:\x:/work:Z`)
	want := "/mnt/c/x:/work:Z"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTranslateBridgeArgv_flagsAndDoubleDash(t *testing.T) {
	in := []string{"--workdir", `C:\repo`, "--", "echo", `C:\nope`}
	out := translateBridgeArgv("d", in)
	if len(out) != 5 {
		t.Fatalf("len %d", len(out))
	}
	if out[0] != "--workdir" || out[1] != "/mnt/c/repo" {
		t.Fatalf("workdir: %q %q", out[0], out[1])
	}
	if out[2] != "--" || out[3] != "echo" || out[4] != `C:\nope` {
		t.Fatalf("after --: %v", out[2:])
	}
}

func TestTranslateBridgeArgv_doubleDashMultipleOnlyFirstSplit(t *testing.T) {
	in := []string{"--workdir", `C:\a`, "--", "cmd", "--flag", `C:\b`}
	out := translateBridgeArgv("d", in)
	if !reflect.DeepEqual(out, []string{"--workdir", "/mnt/c/a", "--", "cmd", "--flag", `C:\b`}) {
		t.Fatalf("got %#v", out)
	}
}

func TestTranslateBridgeArgv_noDoubleDash_allTranslated(t *testing.T) {
	in := []string{"--data-dir", `C:\data`, "--workdir", `D:\w`}
	out := translateBridgeArgv("d", in)
	if !reflect.DeepEqual(out, []string{"--data-dir", "/mnt/c/data", "--workdir", "/mnt/d/w"}) {
		t.Fatalf("got %#v", out)
	}
}

func TestTranslateBridgeArgv_mount(t *testing.T) {
	in := []string{"--mount", `C:\Users\x:/work:rw`}
	out := translateBridgeArgv("d", in)
	if out[1] != "/mnt/c/Users/x:/work:rw" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_twoMounts(t *testing.T) {
	in := []string{"--mount", `C:\a:/a`, "--mount", `D:\b:/b`}
	out := translateBridgeArgv("d", in)
	if out[1] != "/mnt/c/a:/a" || out[3] != "/mnt/d/b:/b" {
		t.Fatalf("got %#v", out)
	}
}

func TestTranslateBridgeArgv_envPath(t *testing.T) {
	in := []string{"--env", `HOST=C:\tmp`}
	out := translateBridgeArgv("d", in)
	if out[1] != "HOST=/mnt/c/tmp" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_envURLValueUnchanged(t *testing.T) {
	in := []string{"--env", `FETCH=https://example.com/x`}
	out := translateBridgeArgv("d", in)
	if out[1] != `FETCH=https://example.com/x` {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_varPath(t *testing.T) {
	in := []string{"--var", `OUT=C:\out.txt`}
	out := translateBridgeArgv("d", in)
	if out[1] != "OUT=/mnt/c/out.txt" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_buildBundleEnvFileWorkPath(t *testing.T) {
	d := "distro"
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"build",
			[]string{"--build", `C:\ctx`},
			[]string{"--build", "/mnt/c/ctx"},
		},
		{
			"bundle-out",
			[]string{"--bundle-out", `C:\x.bundle`},
			[]string{"--bundle-out", "/mnt/c/x.bundle"},
		},
		{
			"env-file",
			[]string{"--env-file", `C:\e\.env`},
			[]string{"--env-file", "/mnt/c/e/.env"},
		},
		{
			"work-path",
			[]string{"--work-path", `C:\sub`},
			[]string{"--work-path", "/mnt/c/sub"},
		},
		{
			"pre-script",
			[]string{"--pre-script", `C:\scripts\a.sh`},
			[]string{"--pre-script", "/mnt/c/scripts/a.sh"},
		},
		{
			"act",
			[]string{"--act", `C:\act.sh`},
			[]string{"--act", "/mnt/c/act.sh"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateBridgeArgv(d, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestTranslateBridgeArgv_isolateDockerImageUnchanged(t *testing.T) {
	in := []string{"--isolate", "ubuntu:22.04"}
	out := translateBridgeArgv("d", in)
	if out[1] != "ubuntu:22.04" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_isolateWindowsPathTranslated(t *testing.T) {
	in := []string{"--isolate", `C:\myctx`}
	out := translateBridgeArgv("d", in)
	if out[1] != "/mnt/c/myctx" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_runURLUnchanged(t *testing.T) {
	in := []string{"--run", "https://example.com/x.yml"}
	out := translateBridgeArgv("d", in)
	if out[1] != "https://example.com/x.yml" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_mntPathNormalized(t *testing.T) {
	in := []string{"--workdir", `/mnt/c/already/linux-style`}
	out := translateBridgeArgv("d", in)
	if out[1] != "/mnt/c/already/linux-style" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_linuxAbsUnchanged(t *testing.T) {
	in := []string{"--workdir", "/home/user/repo"}
	out := translateBridgeArgv("d", in)
	if out[1] != "/home/user/repo" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_initDest(t *testing.T) {
	in := []string{"init", `D:\newproj`}
	out := translateBridgeArgv("d", in)
	if out[1] != "/mnt/d/newproj" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_initTemplateNameUnchanged(t *testing.T) {
	in := []string{"init", "my-workflow"}
	out := translateBridgeArgv("d", in)
	if out[1] != "my-workflow" {
		t.Fatalf("got %q", out[1])
	}
}

func TestTranslateBridgeArgv_initFromSkipsURLAndTranslatesDest(t *testing.T) {
	in := []string{"init", "--from", "https://example.com/tpl", `C:\dest`}
	out := translateBridgeArgv("d", in)
	if !reflect.DeepEqual(out, []string{"init", "--from", "https://example.com/tpl", "/mnt/c/dest"}) {
		t.Fatalf("got %#v", out)
	}
}

func TestTranslateBridgeArgv_actionInitSkipsFrom(t *testing.T) {
	in := []string{"action", "init", "--from", "commit-worktree", `C:\a.sh`}
	out := translateBridgeArgv("d", in)
	if out[3] != "commit-worktree" {
		t.Fatalf("from value: %q", out[3])
	}
	if out[4] != "/mnt/c/a.sh" {
		t.Fatalf("dest: %q", out[4])
	}
}

func TestTranslateBridgeArgv_preInit(t *testing.T) {
	in := []string{"pre", "init", "--from", "x", `C:\p.sh`}
	out := translateBridgeArgv("d", in)
	if out[4] != "/mnt/c/p.sh" {
		t.Fatalf("got %#v", out)
	}
}

func TestTranslateBridgeArgv_templateInit(t *testing.T) {
	in := []string{"template", "init", "--from", "llm-worktree", `C:\w`}
	out := translateBridgeArgv("d", in)
	if out[4] != "/mnt/c/w" {
		t.Fatalf("got %#v", out)
	}
}

func TestTranslateBridgeArgv_actionCreate(t *testing.T) {
	in := []string{"action", "create", `C:\x.sh`}
	out := translateBridgeArgv("d", in)
	if out[2] != "/mnt/c/x.sh" {
		t.Fatalf("got %#v", out)
	}
}

func TestIsProbablyWindowsFilesystemPath(t *testing.T) {
	cases := []struct {
		p    string
		want bool
	}{
		{`C:\x`, true},
		{`\\srv\share`, true},
		{`linux/rel`, false},
		{`/mnt/c/x`, false},
		{"e:/tmp", true},
	}
	for _, tc := range cases {
		if got := isProbablyWindowsFilesystemPath(tc.p); got != tc.want {
			t.Fatalf("isProbablyWindowsFilesystemPath(%q) = %v want %v", tc.p, got, tc.want)
		}
	}
}
