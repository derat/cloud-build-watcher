# cloud-build-watcher

This repository contains a [Cloud Function] that watches for [Cloud Build]
messages via a [Pub/Sub] topic and takes actions in response to them. Currently,
those actions only include sending email notifications.

I wrote this because I was setting up automated testing and deployment using
Cloud Build and was dismayed by how hard it was to do seemingly-simple things
like sending email notifications about failed builds or displaying build status
badges.

[Cloud Function]: https://cloud.google.com/functions
[Cloud Build]: https://cloud.google.com/build
[Pub/Sub]: https://cloud.google.com/pubsub

## Deploying

> Replace `<project-id>` in the following commands with the Google Cloud Project
> ID of the project in which the builds will be performed.

You may need to create a `cloud-builds` Pub/Sub topic first. Cloud Build will
allegedly send messages to it automatically.

```sh
gcloud --project=<project-id> pubsub topics create cloud-builds
```

The `WatchBuilds` Cloud Function can be deployed by running a command similar to
the following.

```sh
gcloud --project=<project-id> functions deploy WatchBuilds \
  --runtime go116 --trigger-topic cloud-builds
```

To see build trigger names and commit hashes, append the following to your YAML
[Cloud Build configurations] \(assuming you don't already have a `tags` section)
to add additional tags to your builds:

```yaml
tags:
  # Set tags to include in Pub/Sub build messages.
  - commit-$COMMIT_SHA
  - trigger-name-$TRIGGER_NAME
```

[Cloud Build configurations]: https://cloud.google.com/build/docs/build-config-file-schema

## Configuration

Configuration is performed via environment variables, which can be passed via
the command line when deploying the function (either via repeated
`--set-env-vars FOO=bar` flags or via the `--env-vars-file` flag) or configured
via the Google Cloud Console (by navigating to your project's Cloud Functions
page, clicking on your function, clicking "Edit", and expanding the "Runtime,
build, connections and security settings" section).

See [Using Environment Variables] for more information about setting environment
variables for Cloud Functions.

[Using Environment Variables]: https://cloud.google.com/functions/docs/configuring/env-var

These variables control **how email is sent**:

*   `EMAIL_HOSTNAME` - Email server hostname, e.g. `smtp.sendgrid.net`
*   `EMAIL_PORT` - Email server port, e.g. `587`
*   `EMAIL_USERNAME` - Email server username, e.g. `apikey`
*   `EMAIL_PASSWORD` - Email server password, e.g. `my-secret-api-key`

These variables control **where email is sent** and related details:

*   `EMAIL_FROM` - Email from address, e.g. `me@example.org` or `My Name
    <me@example.org>`
*   `EMAIL_RECIPIENTS` - Comma-separated email recipients, e.g.
    `user1@example.org, user2@example.org`
*   `EMAIL_TIME_ZONE` - [TZ database name] of time zone to use in email
    messages, e.g. `America/Los_Angeles`, `America/New_York`, or `Europe/Berlin`
    (default is `Etc/UTC`, i.e. +00:00)

These variables control **which build events result in email**:

*   `EMAIL_BUILD_TRIGGER_IDS` - Whitespace- or comma-separated list of Build
    trigger IDs that can produce email, e.g. `123-456, 789-123`
*   `EMAIL_BUILD_TRIGGER_NAMES` - Whitespace- or comma-separated list of Cloud
    Build trigger names that can produce email, e.g.
    `my-trigger, my-other-trigger` (this requires `trigger-name-` tags to be set
    on your builds as described in the "Deploying" section)
*   `EMAIL_BUILD_STATUSES` - Whitespace- or comma-separated list of [build
    statuses] that can produce email (default is
    `FAILURE,INTERNAL_ERROR,TIMEOUT`)

If either `EMAIL_BUILD_TRIGGER_NAMES` or `EMAIL_BUILD_TRIGGER_NAMES` is
supplied, email is only sent for events that were the result of a trigger
matched by either variable.

[TZ database name]: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
[build statuses]: https://pkg.go.dev/google.golang.org/genproto/googleapis/devtools/cloudbuild/v1#Build_Status

## A rant

Google provides a [Configuring SMTP notifications] page with instructions for
setting up email notifications for Cloud Build. It sends you on a whirlwind tour
of Google Cloud products that have nothing to do with build notifications:

*   Save your email password to [Secret Manager].
*   Use [IAM] to find your [Cloud Build] \(or is it [Cloud Run]?) service
    account.
*   Go back to Secret Manager to give the service account access to the
    password.
*   Go back to IAM to give the service account read and write access to [Cloud
    Storage]. (?!)
*   Write a bespoke YAML configuration file for the notifier (which you'll have
    a hard time finding on your computer when you want to change the config
    later).
*   Copy your YAML file to Cloud Storage, maybe creating a new bucket in the
    process (there's no guidance on where you should put it).
*   Deploy a prebuilt [Docker] image containing the notifier to Cloud Run from
    the command line.
*   Run a weird `add-iam-policy-binding` command to give [Pub/Sub] some
    permissions.
*   Create a service account "to represent your Pub/Sub subscription identity".
*   Run another weird `add-iam-policy-binding` command to give more permissions.
*   Create a `cloud-builds` Pub/Sub topic.
*   Create a Pub/Sub push subscriber for the notifier.

You'll also need to learn enough of a Google-designed non-Turing-complete
language called [CEL] to write expressions describing which build events you
want to be notified about.

[Configuring SMTP notifications]: https://cloud.google.com/build/docs/configuring-notifications/configure-smtp
[Secret Manager]: https://cloud.google.com/secret-manager
[IAM]: https://cloud.google.com/iam
[Cloud Run]: https://cloud.google.com/run
[Cloud Storage]: https://cloud.google.com/storage
[Docker]: https://www.docker.com/
[CEL]: https://opensource.google/projects/cel

There's a different [Automating configuration for notifications] page which
contains a subset of the above instructions and points you at the
[cloud-build-notifiers] repo, which contains the source code for the notifier
used on the other page plus a `setup.sh` script that tries to automate many of
the previous steps. It prints a lot of output, including many (harmless?) errors
when I run it.

Assuming it works, you'll end up with bare-bones email notifications about
builds. I tried to [add some more
details](https://github.com/derat/cloud-build-notifiers/commit/1c79a506deda796d6280b0648697bd4f2b1b181b),
but to use my code, I had to build a new Docker container in the (deprecated?)
[Container Registry] and hack the `setup.sh` script to use it.

[Automating configuration for notifications]: https://cloud.google.com/build/docs/configuring-notifications/automate#smtp
[cloud-build-notifiers]: https://github.com/GoogleCloudPlatform/cloud-build-notifiers
[Container Registry]: https://cloud.google.com/container-registry

For some reason, I'm reminded of
["So, how do I query the database?"](http://howfuckedismydatabase.com/nosql/).

Just to mention it, here's how I configured email notifications in [Travis]:

```yaml
notifications:
  email:
    on_success: never
    on_failure: always
```

[Travis]: https://www.travis-ci.com/
