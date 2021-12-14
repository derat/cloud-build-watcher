// Copyright 2021 Daniel Erat.
// All rights reserved.

// Package watch defines a Cloud Function for watching Cloud Build jobs.
package watch

import (
	"context"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/pubsub"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// WatchBuilds is a Cloud Function that processes Pub/Sub messages sent by Cloud Build.
func WatchBuilds(ctx context.Context, msg *pubsub.Message) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed loading config: %v", err)
	}

	var build cbpb.Build
	if err := (protojson.UnmarshalOptions{
		AllowPartial:   true,
		DiscardUnknown: true,
	}).Unmarshal(msg.Data, &build); err != nil {
		return err
	}

	log.Printf("Got message about build %s with status %s", build.Id, build.Status)

	if err := cfg.checkEmail(&build); err != nil {
		log.Print("Not sending email: ", err)
	} else if err := sendEmail(ctx, cfg, &build); err != nil {
		log.Print("Failed sending email: ", err)
	}

	if err := cfg.checkBadge(&build); err != nil {
		log.Print("Not writing badge: ", err)
	} else if err := writeBadge(ctx, cfg, &build); err != nil {
		log.Print("Failed writing badge: ", err)
	}

	return nil
}

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
