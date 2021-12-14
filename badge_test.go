// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

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
