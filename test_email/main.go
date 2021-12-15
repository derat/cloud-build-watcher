// Copyright 2021 Daniel Erat.
// All rights reserved.

// Package main is a simple program for testing the formatting of email messages.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"time"

	watch "github.com/derat/cloud-build-watcher"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <email-address>\n"+
			"Sends an example build notification to the specified address.\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(2)
	}

	to, err := mail.ParseAddress(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Bad email address %q: %v\n", flag.Arg(0), err)
		os.Exit(2)
	}

	var from *mail.Address
	if v := os.Getenv("MAILNAME"); v == "" {
		fmt.Fprintln(os.Stderr, "MAILNAME environment variable not set")
		os.Exit(1)
	} else if from, err = mail.ParseAddress(v); err != nil {
		fmt.Fprintf(os.Stderr, "Failed parsing MAILNAME %q: %v\n", v, err)
		os.Exit(1)
	}

	now := time.Now()
	msg, err := watch.BuildEmail(watch.FakeConfig(from, to), &cbpb.Build{
		ProjectId:      "project-id",
		Id:             "12345-67890",
		LogUrl:         "https://www.example.org/",
		BuildTriggerId: "trigger-id",
		Status:         cbpb.Build_FAILURE,
		StartTime:      tspb.New(now.Add(-3*time.Minute - 41*time.Second)),
		FinishTime:     tspb.New(now),
		Substitutions: map[string]string{
			"BRANCH_NAME":  "branch-name",
			"COMMIT_SHA":   "commit-sha",
			"REPO_NAME":    "repo-name",
			"TRIGGER_NAME": "trigger-name",
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed building email:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Sending email from %v to %v\n", from.Address, to.Address)
	cmd := exec.Command("sendmail", to.Address)
	cmd.Stdin = bytes.NewReader(msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v failed: %v\n", cmd.Args, err)
		os.Exit(1)
	}
}
