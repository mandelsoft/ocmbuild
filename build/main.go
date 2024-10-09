package main

import (
	"fmt"
	"os"

	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	utils "ocm.software/ocm/api/ocm/ocmutils"
	"ocm.software/ocm/cmds/test/build/build"
)

func main() {
	if len(os.Args) > 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <archive> <buildfile>\n", os.Args[0])
		os.Exit(1)
	}
	ctx := clictx.New()

	_, err := utils.Configure(ctx, "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: configuration failed: %s\n", os.Args[0], err.Error())
		os.Exit(1)
	}
	opts := build.Options{
		Create:  true,
		Format:  ctf.FormatDirectory,
		Mode:    0o770,
		Version: "1.0.0",
	}

	if len(os.Args) > 1 {
		opts.Archive = os.Args[1]
	}
	if len(os.Args) > 2 {
		opts.BuildFile = os.Args[2]
	}
	err = build.Execute(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: build failed: %s\n", os.Args[0], err.Error())
		os.Exit(1)
	}
}

func mainOld() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <archive> <constructor>\n", os.Args[0])
		os.Exit(1)
	}
	ctx := clictx.New()

	opts := build.Options{
		Create:  true,
		Archive: os.Args[1],
		Format:  ctf.FormatDirectory,
		Mode:    0o770,
		Version: "1.0.0",
	}
	err := build.Build(ctx, os.Args[2], opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
