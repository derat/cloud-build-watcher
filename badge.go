// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"text/template"

	"cloud.google.com/go/storage"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
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
	w.CacheControl = "no-cache"
	if err := CreateBadge(w, build); err != nil {
		return err
	}
	return w.Close()
}

// CreateBadge creates an SVG badge image for build and writes it to w.
func CreateBadge(w io.Writer, build *cbpb.Build) error {
	right, ok := badgeStatuses[build.Status]
	if !ok {
		return fmt.Errorf("no badge info defined for status %q", build.Status)
	}
	left := badgeLeft
	left.Width = 90 /* from badgeTemplate */ - right.Width

	tmpl, err := template.New("").Parse(badgeTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, struct{ Left, Right badgeInfo }{Left: left, Right: right})
}

const badgeTemplate = `<svg xmlns="http://www.w3.org/2000/svg" width="90" height="20">
  <g font-family="DejaVu Sans,Verdana,Geneva,sans-serif" text-anchor="middle" font-size="10">
    <rect width="90" height="20" rx="3" fill="{{.Left.BG}}" />
    <text x="{{.Left.Center}}" y="14" fill="{{.Left.FG}}">{{.Left.Text}}</text>
    <g transform="translate({{.Left.Width}},0)">
      <rect width="{{.Right.Width}}" height="20" rx="3" fill="{{.Right.BG}}" />
      <path d="M0 0h4v20h-4z" fill="{{.Right.BG}}" />
      <text x="{{.Right.Center}}" y="14" fill="{{.Right.FG}}">{{.Right.Text}}</text>
    </g>
  </g>
</svg>`
