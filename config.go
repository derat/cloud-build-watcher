// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// config contains the Cloud Function's configuration data.
type config struct {
	emailHostname string // server hostname, e.g. "smtp.sendgrid.net"
	emailPort     int    // server port, e.g. 587
	emailUsername string // server username, e.g. "apikey"
	emailPassword string // server password, e.g. "my-secret-api-key"

	emailFrom       *mail.Address   // from address
	emailRecipients []*mail.Address // recipients
	emailTimeZone   *time.Location  // used for dates

	emailBuildTriggerIDs   map[string]struct{} // Cloud Build trigger IDs, empty to not check
	emailBuildTriggerNames map[string]struct{} // Cloud Build trigger names, empty to not check
	emailBuildStatuses     map[string]struct{} // Cloud Build statuses, e.g. "SUCCESS" or "FAILURE"
}

var listRegexp = regexp.MustCompile(`\s*,\s*`)

// loadConfig constructs a new config object from environment variables.
// An error is returned if any variables are unparseable.
func loadConfig() (*config, error) {
	var firstErr error
	saveError := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	strVar := func(n, def string) string {
		return strings.TrimSpace(os.Getenv(n))
	}
	intVar := func(n, def string) int {
		v, err := strconv.Atoi(strVar(n, def))
		saveError(err)
		return v
	}
	listVar := func(n, def string) map[string]struct{} {
		ev := strVar(n, def)
		if len(ev) == 0 {
			return nil
		}
		v := make(map[string]struct{})
		for _, s := range listRegexp.Split(ev, -1) {
			v[s] = struct{}{}
		}
		return v
	}

	// Parse simple fields.
	cfg := config{
		emailHostname:          strVar("EMAIL_HOSTNAME", ""),
		emailPort:              intVar("EMAIL_PORT", "25"),
		emailUsername:          strVar("EMAIL_USERNAME", ""),
		emailPassword:          strVar("EMAIL_PASSWORD", ""),
		emailBuildTriggerIDs:   listVar("EMAIL_BUILD_TRIGGER_IDS", ""),
		emailBuildTriggerNames: listVar("EMAIL_BUILD_TRIGGER_NAMES", ""),
		emailBuildStatuses:     listVar("EMAIL_BUILD_STATUSES", "FAILURE,INTERNAL_ERROR,TIMEOUT"),
	}
	if firstErr != nil {
		return nil, firstErr
	}

	// Parse email addresses.
	var err error
	if v := strVar("EMAIL_FROM", ""); v != "" {
		if cfg.emailFrom, err = mail.ParseAddress(v); err != nil {
			return nil, fmt.Errorf("bad EMAIL_FROM: %v", err)
		}
	}
	if v := strVar("EMAIL_RECIPIENTS", ""); v != "" {
		if cfg.emailRecipients, err = mail.ParseAddressList(v); err != nil {
			return nil, fmt.Errorf("bad EMAIL_RECIPIENTS: %v", err)
		}
	}

	// Load and validate time zone.
	if cfg.emailTimeZone, err = time.LoadLocation(strVar("EMAIL_TIME_ZONE", "Etc/UTC")); err != nil {
		return nil, fmt.Errorf("bad EMAIL_TIME_ZONE: %v", err)
	}

	// Validate build statuses.
	for s := range cfg.emailBuildStatuses {
		if _, ok := cbpb.Build_Status_value[s]; !ok {
			return nil, fmt.Errorf("bad status %q in EMAIL_BUILD_STATUSES", s)
		}
	}

	return &cfg, nil
}

// checkEmail returns nil if an email notification should be sent for b
// per cfg and a descriptive error otherwise.
func (cfg *config) checkEmail(b *cbpb.Build) error {
	if cfg.emailHostname == "" {
		return errors.New("EMAIL_HOSTNAME not set")
	}
	if cfg.emailFrom == nil {
		return errors.New("EMAIL_FROM not set")
	}
	if len(cfg.emailRecipients) == 0 {
		return errors.New("EMAIL_RECIPIENTS not set")
	}
	if len(cfg.emailBuildTriggerIDs) > 0 || len(cfg.emailBuildTriggerNames) > 0 {
		name := buildTag(b, triggerNameTag, "")
		_, idOk := cfg.emailBuildTriggerIDs[b.BuildTriggerId]
		_, nameOk := cfg.emailBuildTriggerNames[name]
		if !idOk && !nameOk {
			return fmt.Errorf("trigger %v (%q) not matched by EMAIL_BUILD_TRIGGER_IDS or "+
				"EMAIL_BUILD_TRIGGER_NAMES", b.BuildTriggerId, name)
		}
	}
	if _, ok := cfg.emailBuildStatuses[b.Status.String()]; !ok {
		return fmt.Errorf("status %q not matched by EMAIL_BUILD_STATUSES", b.Status)
	}
	return nil
}

// emailRecipientsAddrs returns a slice of bare addresses from cfg.emailRecipients.
func (cfg *config) emailRecipientsAddrs() []string {
	addrs := make([]string, len(cfg.emailRecipients))
	for i, a := range cfg.emailRecipients {
		addrs[i] = a.Address
	}
	return addrs
}
