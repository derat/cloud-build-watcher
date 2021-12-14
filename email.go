// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"
	htemplate "text/template"
	ttemplate "text/template"
	"time"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// sendEmail sends an email message describing build per cfg.
// cfg.checkEmail must be called first to check that email should actually be sent.
func sendEmail(ctx context.Context, cfg *Config, build *cbpb.Build) error {
	msg, err := BuildEmail(cfg, build)
	if err != nil {
		return fmt.Errorf("building email: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.emailHostname, cfg.emailPort)
	var auth smtp.Auth
	if cfg.emailUsername != "" {
		auth = smtp.PlainAuth("", cfg.emailUsername, cfg.emailPassword, cfg.emailHostname)
	}

	log.Printf("Sending email to %v", strings.Join(cfg.emailRecipientsAddrs(), ","))
	return smtp.SendMail(addr, auth, cfg.emailFrom.Address, cfg.emailRecipientsAddrs(), msg)
}

// BuildEmail constructs an email message describing build per cfg.
// It is exported so it can be used by the test_email program.
func BuildEmail(cfg *Config, build *cbpb.Build) ([]byte, error) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)

	// Write headers.
	writeHead := func(n, v string) { io.WriteString(&b, n+": "+v+"\r\n") }
	writeHead("From", cfg.emailFrom.String())
	// TODO: Preserve names instead of just using addresses?
	writeHead("To", strings.Join(cfg.emailRecipientsAddrs(), ", "))
	writeHead("Subject",
		fmt.Sprintf("[%s] %s %s (build %s)", build.ProjectId,
			buildTag(build, triggerNameTag, "[unknown]"),
			build.Status, strings.Split(build.Id, "-")[0]))
	writeHead("Date", time.Now().In(cfg.emailTimeZone).Format(time.RFC1123Z))
	writeHead("MIME-Version", "1.0")
	writeHead("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	io.WriteString(&b, "\r\n")

	writeBody := func(ctype string, f func(io.Writer) error) error {
		head := make(textproto.MIMEHeader)
		head.Add("Content-Type", ctype)
		pw, err := mw.CreatePart(head)
		if err != nil {
			return err
		}
		return f(pw)
	}

	const timeFmt = time.RFC1123Z // "Mon, 02 Jan 2006 15:04:05 -0700"
	start := build.StartTime.AsTime()
	end := build.FinishTime.AsTime()
	tdata := struct {
		BuildID     string
		LogURL      string
		TriggerID   string
		TriggerName string
		TriggerURL  string
		Status      string
		Commit      string
		Start       string
		End         string
		Duration    string
	}{
		BuildID:     build.Id,
		LogURL:      build.LogUrl,
		TriggerID:   build.BuildTriggerId,
		TriggerName: buildTag(build, triggerNameTag, ""),
		TriggerURL:  "https://console.cloud.google.com/cloud-build/triggers/edit/" + build.BuildTriggerId,
		Status:      build.Status.String(),
		Commit:      buildTag(build, commitTag, ""),
		Start:       start.In(cfg.emailTimeZone).Format(timeFmt),
		End:         end.In(cfg.emailTimeZone).Format(timeFmt),
		Duration:    formatDuration(end.Sub(start)),
	}

	// Add plain text part.
	if err := writeBody("text/plain; charset=UTF-8", func(w io.Writer) error {
		tmpl, err := ttemplate.New("").Parse(textTemplate)
		if err != nil {
			return err
		}
		return tmpl.Execute(w, tdata)
	}); err != nil {
		return nil, fmt.Errorf("text: %v", err)
	}

	// Add HTML part.
	if err := writeBody("text/html; charset=UTF-8", func(w io.Writer) error {
		tmpl, err := htemplate.New("").Parse(htmlTemplate)
		if err != nil {
			return err
		}
		return tmpl.Execute(w, tdata)
	}); err != nil {
		return nil, fmt.Errorf("HTML: %v", err)
	}

	// Finish up the message.
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

const textTemplate = `
Build:     {{.BuildID}}
{{if .TriggerID -}}
Trigger:   {{or .TriggerName .TriggerID}}
{{end -}}
Status:    {{.Status}}
Commit:    {{.Commit}}
Start:     {{.Start}}
End:       {{.End}}
Duration:  {{.Duration}}
Log:       {{.LogURL}}
`

const htmlTemplate = `
<!DOCTYPE html>
<html>
<body>
<table>
  <tr><td>Build:</td><td><a href="{{.LogURL}}">{{.BuildID}}</a></td></tr>
  {{if .TriggerID -}}
  <tr><td>Trigger:</td><td><a href="{{.TriggerURL}}">{{or .TriggerName .TriggerID}}</a></td></tr>
  {{end -}}
  <tr><td>Status:</td><td>{{.Status}}</td></tr>
  <tr><td>Commit:</td><td>{{.Commit}}</td></tr>
  <tr><td>Start:</td><td>{{.Start}}</td></tr>
  <tr><td>End:</td><td>{{.End}}</td></tr>
  <tr><td>Duration:</td><td>{{.Duration}}</td></tr>
</table>
</body>
</html>
`

// formatDuration formats d as e.g. "4h23m5s", "2m4s", or "14s".
func formatDuration(d time.Duration) string {
	var s string
	if d >= time.Hour {
		s += fmt.Sprintf("%dh", d/time.Hour)
		d %= time.Hour
	}
	if d >= time.Minute {
		s += fmt.Sprintf("%dm", d/time.Minute)
		d %= time.Minute
	}
	if sec := d / time.Second; sec > 0 || s == "" {
		s += fmt.Sprintf("%ds", sec)
	}
	return s
}
