// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"context"
	"errors"
	"fmt"
	htemplate "html/template"
	"io"
	"log"
	"strings"
	ttemplate "text/template"
	"time"

	"cloud.google.com/go/storage"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

const (
	// badgeTimeLayout is the layout for the build start time displayed when hovering over a badge.
	// See the time.Layout documentation at https://pkg.go.dev/time#pkg-constants.
	// TODO: This seems to not actually work except when viewing the image directly.
	// It seems like <img> elements may not receive pointer events.
	badgeTimeLayout = "2 Jan 15:04"

	// After setting an object's CacheControl attribute to 'no-cache' or 'no-store', requests to its
	// https://storage.googleapis.com/ endpoint get responses with Expires headers one year in the
	// future:
	//
	//  Cache-Control: no-cache
	//  Date: Thu, 16 Dec 2021 15:19:59 GMT
	//  Expires: Fri, 16 Dec 2022 15:19:59 GMT
	//
	// The GitHub caching system at https://camo.githubusercontent.com/ seems to serve stale badges
	// as a result. This is discussed in various places:
	//
	//  https://stackoverflow.com/questions/12868505/
	//  https://stackoverflow.com/questions/49708712/
	//  https://issuetracker.google.com/issues/77842189
	//  https://github.com/github/markup/issues/224
	//
	// Setting a short delay instead (this is the one that Travis uses) seems to prevent this:
	//
	//  Cache-Control: max-age=30, s-maxage=30
	//  Date: Thu, 16 Dec 2021 15:44:35 GMT
	//  Expires: Thu, 16 Dec 2021 15:45:05 GMT
	badgeCacheControl = "max-age=30, s-maxage=30"
)

// badgeInfo contains information about how a portion of a badge should be rendered.
type badgeInfo struct {
	Text   string // text to render
	FG, BG string // foreground (text) and background colors as "#rgb" or "#rrggbb"
	Width  int    // width of portion in pixels
}

func (b badgeInfo) Center() int { return b.Width / 2 }

// badgeLeft describes how the left side of the badge should be rendered.
var badgeLeft = badgeInfo{"build", "#fff", "#555", -1 /* inferred from status width */}

// badgeStatuses defines how the right side of the badge should be rendered for different
// build statuses. Statuses not listed here do not result in badge updates.
var badgeStatuses = map[cbpb.Build_Status]badgeInfo{
	cbpb.Build_SUCCESS:        {"success", "#fff", "#2da44e", 52},
	cbpb.Build_FAILURE:        {"failure", "#fff", "#c62828", 52},
	cbpb.Build_INTERNAL_ERROR: {"error", "#000", "#ffeb3b", 52},
	cbpb.Build_TIMEOUT:        {"timeout", "#fff", "#333", 52},
}

// writeBadge writes a badge image describing build per cfg.
// cfg.checkBadge must be called first to check that a badge should actually be written.
func writeBadge(ctx context.Context, cfg *Config, build *cbpb.Build) error {
	if build.BuildTriggerId == "" {
		return errors.New("no build trigger ID")
	}

	name := build.BuildTriggerId + ".svg"
	log.Printf("Writing badge %v to bucket %v", name, cfg.badgeBucket)

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	w := client.Bucket(cfg.badgeBucket).Object(name).NewWriter(ctx)
	w.ContentType = "image/svg+xml"
	w.CacheControl = badgeCacheControl
	if err := CreateBadge(w, build); err != nil {
		return err
	} else if err := w.Close(); err != nil {
		return err
	}

	if cfg.badgeReports {
		rname := build.BuildTriggerId + ".html"
		w := client.Bucket(cfg.badgeBucket).Object(rname).NewWriter(ctx)
		w.ContentType = "text/html; charset=UTF-8"
		w.CacheControl = badgeCacheControl
		if err := CreateReport(w, build); err != nil {
			return err
		} else if err := w.Close(); err != nil {
			return err
		}
	}
	return nil
}

// CreateBadge creates an SVG badge image for build and writes it to w.
func CreateBadge(w io.Writer, build *cbpb.Build) error {
	right, ok := badgeStatuses[build.Status]
	if !ok {
		return fmt.Errorf("no badge info defined for status %q", build.Status)
	}
	left := badgeLeft
	left.Width = 90 /* from badgeTemplate */ - right.Width

	tmpl, err := ttemplate.New("").Parse(strings.TrimSpace(badgeTemplate))
	if err != nil {
		return err
	}
	return tmpl.Execute(w, struct {
		Left, Right badgeInfo
		Date        string
	}{
		Left:  left,
		Right: right,
		Date:  build.StartTime.AsTime().UTC().Format(badgeTimeLayout),
	})
}

const badgeTemplate = `
<svg xmlns="http://www.w3.org/2000/svg" width="90" height="20">
  <g font-family="DejaVu Sans,Verdana,Geneva,sans-serif" text-anchor="middle" font-size="10">
    <rect width="90" height="20" rx="3" fill="{{.Left.BG}}" />
    <text x="{{.Left.Center}}" y="14" fill="{{.Left.FG}}">{{.Left.Text}}</text>
    <g transform="translate({{.Left.Width}},0)">
      <rect width="{{.Right.Width}}" height="20" rx="3" fill="{{.Right.BG}}" />
      <path d="M0 0h4v20h-4z" fill="{{.Right.BG}}" />
      <text x="{{.Right.Center}}" y="14" fill="{{.Right.FG}}">{{.Right.Text}}</text>
    </g>
    <rect width="90" height="20" rx="3" fill="#555" opacity="0">
      <set attributeName="opacity" to="1" begin="over.mouseover" end="over.mouseout" />
    </rect>
    <text x="45" y="14" fill="#fff" opacity="0">{{.Date}}
      <set attributeName="opacity" to="1" begin="over.mouseover" end="over.mouseout" />
    </text>
    <rect id="over" width="90" height="20" opacity="0" />
  </g>
</svg>
`

// CreateReport writes an HTML document with build's status and timing information to w.
func CreateReport(w io.Writer, build *cbpb.Build) error {
	tmpl, err := htemplate.New("").Parse(strings.TrimSpace(reportTemplate))
	if err != nil {
		return err
	}

	const timeFmt = time.RFC1123Z // "Mon, 02 Jan 2006 15:04:05 -0700"
	start := build.StartTime.AsTime()
	end := build.FinishTime.AsTime()
	tdata := struct {
		Status   string
		Start    string
		End      string
		Duration string
	}{
		Status:   build.Status.String(),
		Start:    start.UTC().Format(timeFmt),
		End:      end.UTC().Format(timeFmt),
		Duration: formatDuration(end.Sub(start)),
	}
	return tmpl.Execute(w, tdata)
}

// This is essentially a subset of email.go's htmlTemplate with potentially-sensitive fields removed.
const reportTemplate = `
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Build report</title>
<style>
body {
  font-family: Arial, Helvetica, sans-serif;
}
table {
  border-spacing: 0;
}
td.left {
  font-weight: bold;
  padding-right: 1em;
}
</style>
</head>
<body>
<table>
  <tr><td class="left">Status</td><td>{{.Status}}</td></tr>
  <tr><td class="left">Start</td><td>{{.Start}}</td></tr>
  <tr><td class="left">End</td><td>{{.End}} ({{.Duration}})</td></tr>
</table>
</body>
</html>
`
