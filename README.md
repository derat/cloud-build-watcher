# cloud-build-watcher

[![Build Status](https://storage.googleapis.com/derat-build-badges/b0d628f0-0b7a-4446-9226-8225c610ad82.svg)](https://storage.googleapis.com/derat-build-badges/b0d628f0-0b7a-4446-9226-8225c610ad82.html)

This repository contains a [Cloud Function] that watches for [Cloud Build]
messages via a [Pub/Sub] topic and takes actions in response to them. Currently,
those actions include sending email notifications and writing SVG badge images
(like the one above this paragraph) and brief HTML reports to a Cloud Storage
bucket.

I wrote this because I was setting up automated testing and deployment using
Cloud Build and was dismayed by how hard it was to do seemingly-simple things
like sending email notifications about failed builds or displaying build status
badges.

[Cloud Function]: https://cloud.google.com/functions
[Cloud Build]: https://cloud.google.com/build
[Pub/Sub]: https://cloud.google.com/pubsub

## Deploying

> Replace `$PROJECT_ID` in the following commands with the Google Cloud Project
> ID of the project in which the builds will be performed, or omit the
> `--project` flag if you only have a single project.

You may need to create a `cloud-builds` Pub/Sub topic first. Cloud Build will
allegedly send messages to it automatically.

```sh
gcloud --project=$PROJECT_ID pubsub topics create cloud-builds
```

The `WatchBuilds` Cloud Function can be deployed by running a command similar to
the following. Deploying a Cloud Function can
[take several minutes](https://github.com/firebase/firebase-tools/issues/536).

```sh
gcloud --project=$PROJECT_ID functions deploy WatchBuilds \
  --runtime go116 --trigger-topic cloud-builds
```

[Cloud Build configurations]: https://cloud.google.com/build/docs/build-config-file-schema

If you want to generate badge images, you'll also need to create a
publicly-readable Cloud Storage bucket. I recommend following the "Creating a
bucket" and "Sharing your files" steps from the [Hosting a static website]
document. You may need to give the Cloud Function permission to write to the
bucket if it doesn't already have it.

[Hosting a static website]: https://cloud.google.com/storage/docs/hosting-static-website

## Configuration

Configuration is performed via environment variables, which can be passed via
the command line when deploying the function (via the `--set-env-vars` or
`--env-vars-file` flags) or configured via the Google Cloud Console (by
navigating to your project's Cloud Functions page, clicking the `WatchBuilds`
function, clicking "Edit", and expanding the "Runtime, build, connections and
security settings" section). Note that the `--set-env-vars` flag won't work with
values that contain commas since it interprets commas as delimiters between
variables.

See [Using Environment Variables] for more information about setting environment
variables for Cloud Functions.

[Using Environment Variables]: https://cloud.google.com/functions/docs/configuring/env-var

### How email is sent

| Name             | Description     | Example             | Default |
| :--------------- | :-------------- | :------------------ | :------ |
| `EMAIL_HOSTNAME` | Server hostname | `smtp.sendgrid.net` |         |
| `EMAIL_PORT`     | Server port     | `587`               | `25`    |
| `EMAIL_USERNAME` | Server username | `apikey`            |         |
| `EMAIL_PASSWORD` | Server password | `my-secret-api-key` |         |

At least `EMAIL_HOSTNAME` and `EMAIL_PORT` must be set in order for email to be
sent.

### Where email is sent (and related details)

| Name               | Description                            | Example                                  | Default   |
| :----------------- | :------------------------------------- | :--------------------------------------- | :-------- |
| `EMAIL_FROM`       | "From" address                         | `My Name <me@example.org>`               |           |
| `EMAIL_RECIPIENTS` | List of recipients                     | `user1@example.org, user2@example.org`   |           |
| `EMAIL_TIME_ZONE`  | time zone [TZ database name]           | `America/Los_Angeles` or `Europe/Berlin` | `Etc/UTC` |

`EMAIL_FROM` and `EMAIL_RECIPIENTS` must be set in order for email to be sent.

[TZ database name]: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

### Which build events result in email

| Name                        | Description            | Example              | Default                            |
| :-------------------------- | :--------------------- | :------------------- | :--------------------------------- |
| `EMAIL_BUILD_TRIGGER_IDS`   | [Build trigger] IDs    | `123-456, 789-123`   |                                    |
| `EMAIL_BUILD_TRIGGER_NAMES` | Trigger names or globs | `my-trigger, main-*` |                                    |
| `EMAIL_BUILD_STATUSES`      | [Build statuses]       |                      | `FAILURE, INTERNAL_ERROR, TIMEOUT` |

Items in the three above lists are separated by commas with optional spaces.

If either `EMAIL_BUILD_TRIGGER_IDS` or `EMAIL_BUILD_TRIGGER_NAMES` is supplied,
email is only sent for events originating from a trigger in either list. Globs
in `EMAIL_BUILD_TRIGGER_NAMES` are evaluated using [filepath.Match].

[Build trigger]: https://cloud.google.com/build/docs/triggers
[Build statuses]: https://pkg.go.dev/google.golang.org/genproto/googleapis/devtools/cloudbuild/v1#Build_Status
[filepath.Match]: https://pkg.go.dev/path/filepath#Match

### Badge images and reports

| Name            | Description                         | Example       | Default |
| :-------------- | :---------------------------------- | :------------ | :------ |
| `BADGE_BUCKET`  | Cloud Storage bucket name           | `my-bucket`   |         |
| `BADGE_REPORTS` | Write HTML reports alongside badges | `1` or `true` | `0`     |

If the `BADGE_BUCKET` environment variable is set, SVG images will be written to
Cloud Storage for `SUCCESS`, `FAILURE`, `INTERNAL_ERROR`, and `TIMEOUT` build
statuses.

Badge filenames take the form `<trigger-id>.svg` and can be accessed via a URL
like `https://storage.googleapis.com/<bucket>/<trigger-id>.svg`. To find a
trigger's ID in the Cloud Console, go to Cloud Build, click "Triggers" in the
sidebar, click on the trigger's name, and look for a long hexadecimal-and-dashes
ID in the URL.

If the `BADGE_REPORTS` variable is set to `1` or `true`, brief HTML build
reports will be written to `BADGE_BUCKET` alongside badge images as
`<trigger-id>.html`. These reports intentionally contain minimal information
about builds (to avoid leaking internal details): just status, start time, end
time, and duration.

[Markdown] similar to the following can be used to display a badge image that
links to the corresponding build report:

```md
[![Build Status](https://storage.googleapis.com/<bucket>/<trigger-id>.svg)](https://storage.googleapis.com/<bucket>/<trigger-id>.html)
```

[Markdown]: https://www.markdownguide.org/

### Customizing email notifications

Email notifications can be customized by modifying the `BuildEmail` function and
`textTemplate` and `htmlTemplate` constants in [email.go](./email.go). The
[test_email](./test_email/main.go) program can be used to send example
notifications so you can see how they're rendered by your email client.

### Customizing badge images

Badge images can be customized by modifying `badgeLeft` and `badgeStatuses`
variables and the `badgeTemplate` constant in [badge.go](./badge.go). The
[test_badge](./test_badge/main.go) program can be used to generate badges and
save them locally so you can see what they look like.

### Seeing more information

The Cloud Function logs information about what it's doing, including why it
chooses not to send email about build messages. You can see this information via
the Cloud Console (go to Cloud Functions, click on the `WatchBuilds` function,
and click the "Logs" tab) or by running a command like the following:

```sh
gcloud --project=$PROJECT_ID functions logs read WatchBuilds
```

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
details](https://gist.github.com/derat/fc5b16bff50ccb1380d3a603616c432a), but to
use my code, I had to build a new Docker container in the (deprecated?)
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
