// Copyright 2021 Daniel Erat.
// All rights reserved.

package watch

import (
	"strings"

	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

const (
	triggerNameTag = "trigger-name"
	commitTag      = "commit"
)

// buildTag tries to extract a value from b's tags.
// Given a name "foo", it will look for a tag prefixed with "foo-" and return
// the rest of the tag. If no matching tag is found, def is returned.
// Tags apparently must be matched by "^[\\w][\\w.-]{0,127}$" (per the failure
// message when you try to start a build with an invalid tag).
func buildTag(b *cbpb.Build, name, def string) string {
	pre := name + "-"
	for _, tag := range b.Tags {
		if strings.HasPrefix(tag, pre) {
			return tag[len(pre):]
		}
	}
	return def
}
