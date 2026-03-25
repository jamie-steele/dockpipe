package application

import (
	"fmt"
	"os"

	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"
)

func cmdPackageRead(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageReadUsageText)
		return nil
	}
	if len(args) < 2 {
		return fmt.Errorf(`usage: dockpipe package read <file.tar.gz> <path-in-archive>

example: dockpipe package read release/artifacts/dockpipe-workflow-demo-0.1.0.tar.gz workflows/demo/config.yml`)
	}
	b, err := packagebuild.ReadFileFromTarGz(args[0], args[1])
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}

const packageReadUsageText = `dockpipe package read <file.tar.gz> <path-in-archive>

Reads one file from a gzip tar without extracting the archive to disk (streaming).
Paths use forward slashes as stored in the tar (e.g. workflows/mywf/config.yml, resolvers/claude/profile).

Use this to inspect package tarballs from dockpipe package build store, or to feed tools without a full unpack.

`
