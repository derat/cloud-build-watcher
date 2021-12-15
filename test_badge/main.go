// Copyright 2021 Daniel Erat.
// All rights reserved.

// Package main is a simple program for testing the rendering of badge images.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	report := flag.Bool("report", false, "Write report file with .html extension alongside image")
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

	now := time.Now()
	build := &cbpb.Build{
		Status:     cbpb.Build_Status(st),
		StartTime:  tspb.New(now.Add(-3*time.Minute - 41*time.Second)),
		FinishTime: tspb.New(now),
	}

	if err := writeFile(flag.Arg(0), func(w io.Writer) error {
		return watch.CreateBadge(w, build)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "Failed writing badge:", err)
		os.Exit(1)
	}

	if *report {
		// Replace the image path's extension (if any) with .html.
		p := flag.Arg(0)
		if ext := filepath.Ext(p); ext != "" {
			p = p[:len(p)-len(ext)]
		}
		p += ".html"

		if err := writeFile(p, func(w io.Writer) error {
			return watch.CreateReport(w, build)
		}); err != nil {
			fmt.Fprintln(os.Stderr, "Failed writing report:", err)
			os.Exit(1)
		}
	}
}

// writeFile creates a file at p and passes it to fn.
func writeFile(p string, fn func(w io.Writer) error) error {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := fn(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
