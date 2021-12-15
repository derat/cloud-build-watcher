// Copyright 2021 Daniel Erat.
// All rights reserved.

// Package main is a simple program for testing the rendering of badge images.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	watch "github.com/derat/cloud-build-watcher"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <output-path>\n"+
			"Writes an SVG badge image to the supplied path.\n", os.Args[0])
		flag.PrintDefaults()
	}
	status := flag.String("status", "SUCCESS", "Build status (SUCCESS, FAILURE, INTERNAL_ERROR, or TIMEOUT)")
	flag.Parse()
	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	st, ok := cbpb.Build_Status_value[*status]
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid build status %q\n", *status)
		os.Exit(2)
	}

	build := &cbpb.Build{
		Status:    cbpb.Build_Status(st),
		StartTime: tspb.New(time.Now()),
	}

	f, err := os.OpenFile(flag.Arg(0), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed opening output file:", err)
		os.Exit(1)
	}
	if err := watch.CreateBadge(f, build); err != nil {
		fmt.Fprintln(os.Stderr, "Failed writing badge:", err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed closing output file:", err)
		os.Exit(1)
	}
}
