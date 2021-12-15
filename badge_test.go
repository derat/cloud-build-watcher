// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"golang.org/x/net/html"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

func TestCreateBadge(t *testing.T) {
	// Just check that the template produces valid XML that contains the status.
	var b bytes.Buffer
	if err := CreateBadge(&b, &cbpb.Build{Status: cbpb.Build_SUCCESS}); err != nil {
		t.Fatal("CreateBadge failed: ", err)
	}
	if err := xml.Unmarshal(b.Bytes(), new(interface{})); err != nil {
		t.Fatalf("Badge isn't valid XML: %v\n%v", err, b.String())
	}
	if want := "success"; !strings.Contains(b.String(), want) {
		t.Errorf("%q doesn't appear in badge:\n%v", want, b.String())
	}
}

func TestCreateReport(t *testing.T) {
	// Check that the template produces valid HTML that contains the status.
	var b bytes.Buffer
	if err := CreateReport(&b, &cbpb.Build{
		Status:     cbpb.Build_SUCCESS,
		StartTime:  makeTimestamp("2021-12-11T19:42:31Z"),
		FinishTime: makeTimestamp("2021-12-11T20:04:51Z"),
	}); err != nil {
		t.Fatal("CreateReport failed: ", err)
	}
	report := b.String() // save since html.Parse advances the buffer
	if _, err := html.Parse(&b); err != nil {
		t.Fatalf("Report isn't valid HTML: %v\n%v", err, b.String())
	}
	if !strings.Contains(report, cbpb.Build_SUCCESS.String()) {
		t.Errorf("%q doesn't appear in report:\n%v", cbpb.Build_SUCCESS.String(), report)
	}
}
