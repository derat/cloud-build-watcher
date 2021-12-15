// Copyright 2021 Daniel Erat.
// All rights reserved.

// Package watch defines a Cloud Function for watching Cloud Build jobs.
package watch

import (
	"context"
	"fmt"
	"log"

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
	// Substitution names to pass to buildSub:
	// https://cloud.google.com/build/docs/configuring-builds/substitute-variable-values
	branchSub      = "BRANCH_NAME"
	commitSub      = "COMMIT_SHA"
	repoSub        = "REPO_NAME"
	triggerNameSub = "TRIGGER_NAME"
)

// buildSub returns the named value from b's Substitutions map.
// If the named substitution does not exist, def is returned instead.
func buildSub(b *cbpb.Build, name, def string) string {
	if v, ok := b.Substitutions[name]; ok {
		return v
	}
	return def
}
