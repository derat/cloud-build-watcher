// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// setEnv sets environment variables based on NAME=value pairs in vars.
// The returned function should be deferred to unset the variables.
// Original values are not preserved, so this should only be used for variables
// specific to this codebase that wouldn't already be set when running tests.
func setEnv(vars []string) (undo func()) {
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		os.Setenv(parts[0], parts[1])
	}
	return func() {
		for _, v := range vars {
			os.Unsetenv(strings.SplitN(v, "=", 2)[0])
		}
	}
}

func TestLoadConfig(t *testing.T) {
	undo := setEnv([]string{
		"EMAIL_HOSTNAME=mail.example.org",
		"EMAIL_PORT=587",
		"EMAIL_USERNAME=user",
		"EMAIL_PASSWORD=pass",
		"EMAIL_FROM=Cloud Build <build@example.org>",
		`EMAIL_RECIPIENTS=user1@example.org,user2@example.org, "Some User" <user3@example.org>`,
		"EMAIL_TIME_ZONE=America/New_York",
		"EMAIL_BUILD_TRIGGER_IDS=123-456,789-012",
		"EMAIL_BUILD_TRIGGER_NAMES=trigger-1, trigger-2",
		"EMAIL_BUILD_STATUSES=FAILURE,TIMEOUT",
	})
	defer undo()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal("loadConfig failed: ", err)
	}

	// Check that the loaded data matches what we set in the environment.
	const wantSrv = "mail.example.org:587:user:pass"
	if got := fmt.Sprintf("%s:%d:%s:%s", cfg.emailHostname, cfg.emailPort,
		cfg.emailUsername, cfg.emailPassword); got != wantSrv {
		t.Errorf("Got email config %v; want %v", got, wantSrv)
	}
	const wantFrom = `"Cloud Build" <build@example.org>`
	if got := cfg.emailFrom.String(); got != wantFrom {
		t.Errorf("Got email from-address %q; want %q", got, wantFrom)
	}
	const wantRecips = "user1@example.org,user2@example.org,user3@example.org"
	if got := strings.Join(cfg.emailRecipientsAddrs(), ","); got != wantRecips {
		t.Errorf("Got email recipients %q; want %q", got, wantRecips)
	}
	const wantTimeZone = "America/New_York"
	if got := cfg.emailTimeZone.String(); got != wantTimeZone {
		t.Errorf("Got email time zone %v; want %v", got, wantTimeZone)
	}
	var wantTriggerIDs = map[string]struct{}{"123-456": {}, "789-012": {}}
	if !reflect.DeepEqual(cfg.emailBuildTriggerIDs, wantTriggerIDs) {
		t.Errorf("Got email trigger IDs %v; want %v",
			cfg.emailBuildTriggerIDs, wantTriggerIDs)
	}
	var wantTriggerNames = map[string]struct{}{"trigger-1": {}, "trigger-2": {}}
	if !reflect.DeepEqual(cfg.emailBuildTriggerNames, wantTriggerNames) {
		t.Errorf("Got email trigger names %v; want %v",
			cfg.emailBuildTriggerIDs, wantTriggerNames)
	}
	var wantStatuses = map[string]struct{}{"FAILURE": {}, "TIMEOUT": {}}
	if !reflect.DeepEqual(cfg.emailBuildStatuses, wantStatuses) {
		t.Errorf("Got email statuses %v; want %v", cfg.emailBuildStatuses, wantStatuses)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal("loadConfig failed: ", err)
	}
	if cfg.emailPort <= 0 {
		t.Error("No default port")
	}
	if len(cfg.emailBuildStatuses) <= 0 {
		t.Error("No default build statuses")
	}
}

func TestConfig_checkEmail(t *testing.T) {
	const (
		host  = "EMAIL_HOSTNAME=mail.example.org"
		port  = "EMAIL_PORT=587"
		from  = "EMAIL_FROM=sender@example.org"
		rcpt  = "EMAIL_RECIPIENTS=recip@example.org"
		ids   = "EMAIL_BUILD_TRIGGER_IDS=123,456"
		names = "EMAIL_BUILD_TRIGGER_NAMES=trigger-1,trigger-2"
		glob  = "EMAIL_BUILD_TRIGGER_NAMES=*1"
	)

	success := &cbpb.Build{Status: cbpb.Build_SUCCESS}
	successID := &cbpb.Build{Status: cbpb.Build_SUCCESS, BuildTriggerId: "123"}
	fail := &cbpb.Build{Status: cbpb.Build_FAILURE}
	failID := &cbpb.Build{Status: cbpb.Build_FAILURE, BuildTriggerId: "123"}
	failBadID := &cbpb.Build{Status: cbpb.Build_FAILURE, BuildTriggerId: "000"}
	failName := &cbpb.Build{
		Status:        cbpb.Build_FAILURE,
		Substitutions: map[string]string{triggerNameSub: "trigger-1"},
	}
	failBadName := &cbpb.Build{
		Status:        cbpb.Build_FAILURE,
		Substitutions: map[string]string{triggerNameSub: "bad-trigger"},
	}

	for _, tc := range []struct {
		env   []string
		build *cbpb.Build
		want  bool // true for nil, false for error
		desc  string
	}{
		{[]string{}, fail, false, "no config"},
		{[]string{port, from, rcpt}, fail, false, "no hostname"},
		{[]string{host, port, rcpt}, fail, false, "no from"},
		{[]string{host, port, from}, fail, false, "no rcpt"},
		{[]string{host, from, rcpt}, fail, true, "default port"},
		{[]string{host, port, from, rcpt}, success, false, "unmatched status"},
		{[]string{host, port, from, rcpt}, fail, true, "matched status"},
		{[]string{host, port, from, rcpt, ids}, fail, false, "no trigger ID"},
		{[]string{host, port, from, rcpt, ids}, failBadID, false, "wrong trigger ID"},
		{[]string{host, port, from, rcpt, ids}, failID, true, "trigger ID matched"},
		{[]string{host, port, from, rcpt, ids}, successID, false, "trigger ID matched, unmatched status"},
		{[]string{host, port, from, rcpt, names}, fail, false, "no trigger name"},
		{[]string{host, port, from, rcpt, names}, failID, false, "ID but no trigger name"},
		{[]string{host, port, from, rcpt, names}, failBadName, false, "wrong trigger name"},
		{[]string{host, port, from, rcpt, names}, failName, true, "trigger name matched"},
		{[]string{host, port, from, rcpt, ids, names}, failName, true, "trigger name matched, no ID"},
		{[]string{host, port, from, rcpt, ids, names}, failID, true, "trigger ID matched, no name"},
		{[]string{host, port, from, rcpt, glob}, failName, true, "trigger name matched by glob"},
		{[]string{host, port, from, rcpt, glob}, failBadName, false, "trigger name not matched by glob"},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer setEnv(tc.env)()
			cfg, err := loadConfig()
			if err != nil {
				t.Fatal("loadConfig failed: ", err)
			}
			if err := cfg.checkEmail(tc.build); err == nil && !tc.want {
				t.Error("checkEmail returned nil; want an error")
			} else if err != nil && tc.want {
				t.Errorf("checkEmail returned %q; want nil", err)
			}
		})
	}
}
