// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"net/mail"
	"regexp"
	"testing"
	"time"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildEmail(t *testing.T) {
	cfg := &config{
		emailFrom: &mail.Address{Name: "Sender Name", Address: "sender@example.org"},
		emailRecipients: []*mail.Address{
			&mail.Address{Name: "Recipient 1", Address: "user1@example.org"},
			&mail.Address{Address: "user2@example.org"},
		},
	}

	makeTimestamp := func(s string) *tspb.Timestamp {
		tt, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("Failed parsing %q: %v", s, err)
		}
		return tspb.New(tt)
	}
	build := &cbpb.Build{
		Id:             "1234-5678",
		ProjectId:      "my-project",
		BuildTriggerId: "trigger-id",
		Status:         cbpb.Build_FAILURE,
		LogUrl:         "https://example.org/log",
		StartTime:      makeTimestamp("2021-12-11T19:42:31Z"),
		FinishTime:     makeTimestamp("2021-12-11T20:04:51Z"),
		Tags: []string{
			"branch-my-branch",
			"commit-my-commit",
			"trigger-name-my-trigger",
		},
	}

	msg, err := buildEmail(cfg, build)
	if err != nil {
		t.Fatal("buildEmail failed: ", err)
	}
	for _, re := range []string{
		`From: "Sender Name" <sender@example\.org>\r\n`,
		`To: user1@example\.org, user2@example\.org\r\n`,
		`Subject: \[my-project\] my-trigger FAILURE \(build 1234\)\r\n`,
		`Build:\s+1234-5678\n`,
		`Trigger:\s+my-trigger\n`,
		`Status:\s+FAILURE\n`,
		`Commit:\s+my-commit\n`,
		`Duration:\s+22m20s\n`,
		`Log:\s+https://example.org/log\n`,
		`<tr><td>Build:</td><td><a href="https://example.org/log">1234-5678</a></td></tr>\n`,
		`<tr><td>Trigger:</td><td><a href="https://console.cloud.google.com/cloud-build/` +
			`triggers/edit/trigger-id">my-trigger</a></td></tr>\n`,
		`<tr><td>Status:</td><td>FAILURE</td></tr>\n`,
		`<tr><td>Commit:</td><td>my-commit</td></tr>\n`,
		`<tr><td>Duration:</td><td>22m20s</td></tr>\n`,
	} {
		if !regexp.MustCompile(re).Match(msg) {
			t.Errorf("buildEmail output not matched by %q", re)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{3*time.Hour + 23*time.Minute + 10*time.Second, "3h23m10s"},
		{3*time.Hour + 23*time.Minute, "3h23m"},
		{3*time.Hour + 10*time.Second, "3h10s"},
		{23*time.Minute + 10*time.Second, "23m10s"},
		{23 * time.Minute, "23m"},
		{53 * time.Second, "53s"},
		{0, "0s"},
	} {
		if got := formatDuration(tc.d); got != tc.want {
			t.Errorf("formatDuration() = %q; want %q", got, tc.want)
		}
	}
}
