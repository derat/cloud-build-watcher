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

	return nil
}
