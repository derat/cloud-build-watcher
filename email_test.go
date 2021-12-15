// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"fmt"
	"log"
	"net/mail"
	"regexp"
	"testing"
	"time"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	tspb "google.golang.org/protobuf/types/known/timestamppb"
)

func makeTimestamp(s string) *tspb.Timestamp {
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Panicf("Failed parsing %q: %v", s, err)
	}
	return tspb.New(tt)
}

func TestBuildEmail(t *testing.T) {
	cfg := &Config{
		emailFrom: &mail.Address{Name: "Sender Name", Address: "sender@example.org"},
		emailRecipients: []*mail.Address{
			&mail.Address{Name: "Recipient 1", Address: "user1@example.org"},
			&mail.Address{Address: "user2@example.org"},
		},
	}
	var err error
	if cfg.emailTimeZone, err = time.LoadLocation("America/New_York"); err != nil {
		t.Fatal("Failed loading time zone: ", err)
	}

	build := &cbpb.Build{
		Id:             "1234-5678",
		ProjectId:      "my-project",
		BuildTriggerId: "trigger-id",
		Status:         cbpb.Build_FAILURE,
		LogUrl:         "https://example.org/log",
		StartTime:      makeTimestamp("2021-12-11T19:42:31Z"),
		FinishTime:     makeTimestamp("2021-12-11T20:04:51Z"),
		Substitutions: map[string]string{
			branchSub:      "my-branch",
			commitSub:      "my-commit",
			repoSub:        "my-repo",
			triggerNameSub: "my-trigger",
		},
	}

	msg, err := BuildEmail(cfg, build)
	if err != nil {
		t.Fatal("BuildEmail failed: ", err)
	}
	for _, re := range []string{
		`From: "Sender Name" <sender@example\.org>\r\n`,
		`To: user1@example\.org, user2@example\.org\r\n`,
		`Subject: \[my-project\] my-trigger FAILURE \(build 1234\)\r\n`,
		`Date: .+ -0500\r\n`,
		`Build:\s+1234-5678\n`,
		`Trigger:\s+my-trigger\n`,
		`Status:\s+FAILURE\n`,
		`Repo:\s+my-repo\n`,
		`Commit:\s+my-commit\n`,
		`Branch:\s+my-branch\n`,
		`Start:\s+Sat, 11 Dec 2021 14:42:31 -0500\n`,
		`End:\s+Sat, 11 Dec 2021 15:04:51 -0500 \(22m20s\)\n`,
		`Log:\s+https://example.org/log\r\n`,
		`<tr><td[^>]*>Build</td><td><a href="https://example.org/log">1234-5678</a></td></tr>\n`,
		`<tr><td[^>]*>Trigger</td><td><a href="https://console.cloud.google.com/cloud-build/` +
			`triggers/edit/trigger-id">my-trigger</a></td></tr>\n`,
		`<tr><td[^>]*>Status</td><td>FAILURE</td></tr>\n`,
		`<tr><td[^>]*>Repo</td><td>my-repo</td></tr>\n`,
		`<tr><td[^>]*>Commit</td><td>my-commit</td></tr>\n`,
		`<tr><td[^>]*>Branch</td><td>my-branch</td></tr>\n`,
		`<tr><td[^>]*>Start</td><td>Sat, 11 Dec 2021 14:42:31 -0500</td></tr>\n`,
		`<tr><td[^>]*>End</td><td>Sat, 11 Dec 2021 15:04:51 -0500 \(22m20s\)</td></tr>\n`,
	} {
		if !regexp.MustCompile(re).Match(msg) {
			t.Errorf("BuildEmail output not matched by %q", re)
		}
	}
	if t.Failed() {
		fmt.Println(string(msg))
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
