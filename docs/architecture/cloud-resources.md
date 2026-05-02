# Cloud Resource Inventory

Complete reference for all GCP and Firebase resources, IAM configuration, secrets, environment variables, and operational commands for the Milton Meeting Summarizer.

---

## GCP Project

| Property | Value |
|---|---|
| Project ID | `miltonmeetingsummarizer` |
| Firebase Project | `miltonmeetingsummarizer` |
| Default region | `us-central1` |
| Project number | Retrieved at setup time: `gcloud projects describe $PROJECT_ID --format='value(projectNumber)'` |

---

## Compute

### Cloud Function: `youtube-webhook`

| Property | Value |
|---|---|
| Generation | Gen 2 (backed by Cloud Run) |
| Runtime | `go124` (Go 1.24) |
| Region | `us-central1` |
| Entry point | `YouTubeWebhook` |
| Source | Repo root (`.`) — Go module `github.com/kfiles/transcriptsummarizer` |
| Trigger | HTTP (unauthenticated) |
| Memory | 512 MiB |
| Timeout | 540 seconds (9 minutes) |
| Service account | `transcript-summarizer@<PROJECT_ID>.iam.gserviceaccount.com` |

**Why unauthenticated?** The YouTube PubSubHubbub hub sends push notifications from Google's infrastructure without any auth headers that Cloud Function identity checks would accept. The endpoint must be publicly accessible.

**Why 540s timeout?** Processing a long meeting video includes: transcript fetch (up to 30s), OpenAI summarization (up to 60s), GCS upload, and Pub/Sub publish — multiplied across multiple videos in a playlist scan. The 9-minute timeout provides headroom for circuit-breaker-bounded batch processing.

---

## Storage

### Firestore Database: `meetingtranscripts`

| Property | Value |
|---|---|
| Type | Firestore in **MongoDB compatibility mode** |
| Access protocol | MongoDB Wire Protocol (port 27017) |
| Connection | Via `MONGODB_URI` secret → `go.mongodb.org/mongo-driver/v2` |
| Database name | `meetingtranscripts` |

#### Collections

| Collection | `_id` Key | Schema Fields |
|---|---|---|
| `playlists` | `playlistId` (YouTube playlist ID) | `channelId`, `title`, `updatedAt`, `pageToken`, `numEntries` |
| `videos` | `videoId` (YouTube video ID) | `playlistId`, `title`, `description`, `position`, `publishedAt` |
| `transcripts` | `videoId + "_" + languageCode` | `videoId`, `languageCode`, `retrievedText`, `summaryText` |
| `officials` | committee name (normalized) | `name`, `members: []string` |

#### Operational Notes
- Max operation timeout: **55 seconds** — enforced by `capCtx()` in [pkg/db/session.go](../../pkg/db/session.go). Firestore rejects `maxTimeMS > 60000`.
- `cmd/officials` uses **drop-then-insert** for the officials collection because Firestore's MongoDB compat layer does not support `BulkWrite`.
- A video's presence in the `videos` collection is the **idempotency guard** for the pipeline. Once inserted, the video is never re-processed.

### GCS Bucket: `miltonmeetingsummarizer-hugo-content`

| Property | Value |
|---|---|
| Location | `us-central1` |
| Storage class | Standard |
| Access control | Uniform bucket-level access |
| Object path pattern | `minutes/{YYYY}/{Month}/{videoId}.md` |
| Contents | Hugo-formatted Markdown meeting summaries |
| Writer | Cloud Function (`roles/storage.objectCreator`) |
| Reader | Cloud Build SA (via `gsutil rsync`) |

The bucket serves as a persistent content staging area. Even if the Cloud Function is invoked multiple times, the bucket always reflects the complete set of processed videos.

---

## Messaging

### Pub/Sub Topic: `youtube-pipeline-trigger`

| Property | Value |
|---|---|
| Project | `miltonmeetingsummarizer` |
| Topic name | `youtube-pipeline-trigger` |
| Publisher | Cloud Function (`roles/pubsub.publisher`) |
| Subscriber | Cloud Build trigger `hugo-build-and-deploy` |
| Message payload | `{"videoId": "<id>"}` or `{"videoId": "all"}` |

The message payload is not used by Cloud Build — it is a signal only. Cloud Build syncs from GCS regardless of which video triggered the build.

---

## Scheduling

### Cloud Scheduler Job: `renew-youtube-subscription`

| Property | Value |
|---|---|
| Location | `us-central1` |
| Schedule | `0 9 */9 * *` (every 9 days at 09:00 UTC) |
| Target URL | `https://pubsubhubbub.appspot.com/subscribe` |
| HTTP method | `POST` |
| Content-Type | `application/x-www-form-urlencoded` |
| Body | `hub.callback=<functionURL>&hub.topic=<feedURL>&hub.mode=subscribe&hub.verify=async` |
| Feed URL | `https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCGnv43oWpciURP-bTDc3GnA` |

**Rationale:** YouTube PubSubHubbub subscriptions expire after approximately 10 days. Renewing every 9 days provides a 1-day buffer against clock drift or scheduler delays, ensuring continuous push notification coverage.

---

## CI/CD

### Cloud Build Trigger: `hugo-build-and-deploy`

| Property | Value |
|---|---|
| Trigger type | Pub/Sub |
| Topic | `projects/<PROJECT_ID>/topics/youtube-pipeline-trigger` |
| GitHub repository | Connected via 1st-gen GitHub App |
| Branch | `main` |
| Build config | `cloudbuild.yaml` (repo root) |
| Service account | `transcript-summarizer` SA (for log writing) |
| Logging | `CLOUD_LOGGING_ONLY` |

**Substitutions:**

| Variable | Value |
|---|---|
| `_CONTENT_BUCKET` | `miltonmeetingsummarizer-hugo-content` |
| `_FIREBASE_PROJECT` | `miltonmeetingsummarizer` |

**Build steps** (from [cloudbuild.yaml](../../cloudbuild.yaml)):

| Step ID | Image | Command | Waits For |
|---|---|---|---|
| `mk-contentdir` | `busybox` | `mkdir -p site/content/minutes` | *(start)* |
| `sync-content` | `gcr.io/google.com/cloudsdktool/cloud-sdk` | `gsutil -m rsync -r gs://$_CONTENT_BUCKET/minutes/ site/content/minutes/` | `mk-contentdir` |
| `hugo-build` | `hugomods/hugo:debian-std-node-lts-0.161.1` | `hugo --source=site --minify` | `sync-content` |
| `install-firebase` | `node` | `npm install firebase-tools` | `hugo-build` |
| `firebase-deploy` | `node` | `./node_modules/.bin/firebase deploy --only hosting --project $_FIREBASE_PROJECT --non-interactive` | `install-firebase` |

### Cloud Build Trigger: `transcript-summarizer-push`

| Property | Value |
|---|---|
| Trigger type | GitHub push |
| Branch | `main` |
| Purpose | Code CI (not part of meeting pipeline); also used as a reference to resolve GitHub owner/repo name during infra setup |

---

## Hosting

### Firebase Hosting: `miltonmeetingsummarizer`

| Property | Value |
|---|---|
| Firebase project | `miltonmeetingsummarizer` |
| Site ID | `miltonmeetingsummarizer` |
| URL | `https://miltonmeetingsummarizer.web.app` |
| Hugo baseURL | `https://miltonmeetingsummarizer.web.app` |
| Public directory | `site/public` |
| Hugo theme | `hugo-geekdoc` |
| Content root | `site/content/minutes/YYYY/Month/{videoId}.md` |
| URL pattern | `/minutes/YYYY/Month/{videoId}/` |

Configuration: [firebase.json](../../firebase.json), [site/hugo.toml](../../site/hugo.toml)

---

## IAM

### Service Account: `transcript-summarizer`

Full email: `transcript-summarizer@<PROJECT_ID>.iam.gserviceaccount.com`

Used by: Cloud Function (`youtube-webhook`) and Cloud Build trigger (`hugo-build-and-deploy` runner)

#### Project-Level Role Bindings

| Role | Condition | Reason |
|---|---|---|
| `roles/storage.objectCreator` | None | Cloud Function uploads Markdown files to GCS |
| `roles/pubsub.publisher` | None | Cloud Function publishes build trigger messages |
| `roles/logging.logWriter` | None | Cloud Function (acting as Cloud Build runner) writes build logs to Cloud Logging |
| `roles/datastore.user` | `resource.name == 'projects/<PROJECT_ID>/databases/meetingtranscripts'` | Read/write Firestore meetingtranscripts DB only. Conditional binding required by project IAM policy. |
| `roles/firebase.admin` | None | Firebase Admin SDK access (Firestore, Auth, Hosting admin) |

#### Secret-Level Bindings (`roles/secretmanager.secretAccessor`)

| Secret | Reason |
|---|---|
| `mongodb-uri` | Connect to Firestore via MongoDB Wire Protocol |
| `openai-api-key` | Call OpenAI GPT-4.1-mini for summarization |
| `facebook-page-id` | Identify Facebook Page for posting |
| `facebook-page-token` | Authenticate Facebook Graph API calls |
| `youtube-api-key` | Call YouTube Data API (playlist scanning) |
| `supadata-api-key` | Fallback transcript provider |
| `transcriptapi-api-key` | Primary transcript provider |
| `firebase-api-token` | *(referenced in setup.sh; see note below)* |

> **Note:** `firebase-api-token` appears in the secret accessor grant loop in `infra/setup.sh` but the secret itself is named `firebase-ci-token`. Verify this is current if you re-run setup.

### Service Account: Cloud Build Default SA

Full email: `<PROJECT_NUMBER>@cloudbuild.gserviceaccount.com`

#### Project-Level Role Bindings

| Role | Condition | Reason |
|---|---|---|
| `roles/firebasehosting.admin` | None | Deploy to Firebase Hosting in `firebase deploy` step |

#### Secret-Level Bindings (`roles/secretmanager.secretAccessor`)

| Secret | Reason |
|---|---|
| `firebase-ci-token` | Firebase CLI uses this token for non-interactive deploys in Cloud Build |
| `mongodb-uri` | Integration tests (when run in Cloud Build context) |

---

## Secrets

All secrets use `--replication-policy=automatic` and are created by `infra/setup.sh`.

| Secret Name | Env Var in Function | Description | Consumers |
|---|---|---|---|
| `mongodb-uri` | `MONGODB_URI` | Firestore MongoDB API connection string (includes hostname, credentials, TLS params) | Cloud Function, admin CLI tools |
| `openai-api-key` | `CHATGPT_API_KEY` | OpenAI API secret key. Used by `pkg/summarize` and `pkg/officials` (LLM fallback) | Cloud Function |
| `facebook-page-id` | `FACEBOOK_PAGE_ID` | Numeric Facebook Page ID for the Milton town page | Cloud Function |
| `facebook-page-token` | `FACEBOOK_PAGE_TOKEN` | Long-lived Facebook Page access token | Cloud Function |
| `youtube-api-key` | `YOUTUBE_API_KEY` | Google Cloud API key with YouTube Data API v3 enabled. No OAuth required. | Cloud Function, admin CLI tools |
| `supadata-api-key` | `SUPADATA_API_KEY` | Supadata transcript service API key | Cloud Function |
| `transcriptapi-api-key` | `TRANSCRIPTAPI_API_KEY` | TranscriptAPI.com API key | Cloud Function |
| `firebase-ci-token` | *(not an env var; used by firebase CLI)* | Firebase CI token from `firebase login:ci`. Required for non-interactive `firebase deploy` in Cloud Build. | Cloud Build |

**To rotate a secret:**
```bash
echo -n "new-value" | gcloud secrets versions add SECRET_NAME --data-file=- --project=miltonmeetingsummarizer
```

After rotating `mongodb-uri`, `openai-api-key`, `facebook-page-id`, `facebook-page-token`, `youtube-api-key`, `supadata-api-key`, or `transcriptapi-api-key`, redeploy the Cloud Function so it picks up the new version (`make deploy`). Secrets are injected at deploy time, not pulled at runtime.

---

## Environment Variables

All env vars below are set on the Cloud Function at deploy time via `--set-env-vars` or `--set-secrets`.

### Set via `--set-env-vars`

| Variable | Value (production) | Description |
|---|---|---|
| `HUGO_CONTENT_DIR` | `/tmp/hugo-content/minutes` | Local directory where Markdown files are written before GCS upload |
| `GCS_BUCKET` | `miltonmeetingsummarizer-hugo-content` | Target GCS bucket for Markdown files |
| `PUBSUB_PROJECT` | `miltonmeetingsummarizer` | GCP project for Pub/Sub client |
| `PUBSUB_TOPIC` | `youtube-pipeline-trigger` | Pub/Sub topic to signal Cloud Build |
| `TRANSCRIPT_PROVIDER` | `transcriptapi` | Selects transcript provider: `transcriptapi`, `supadata`, `youtube`, or empty (→ supadata) |
| `PROJECT_ID` | `miltonmeetingsummarizer` | Used to construct Firebase Hosting URL for Facebook posts |
| `FACEBOOK_ENABLED` | `true` | Set to `false` to disable Facebook posting without removing credentials |

### Set via `--set-secrets` (injected as env vars)

| Variable | Source Secret | Latest version |
|---|---|---|
| `MONGODB_URI` | `mongodb-uri` | `latest` |
| `CHATGPT_API_KEY` | `openai-api-key` | `latest` |
| `FACEBOOK_PAGE_ID` | `facebook-page-id` | `latest` |
| `FACEBOOK_PAGE_TOKEN` | `facebook-page-token` | `latest` |
| `YOUTUBE_API_KEY` | `youtube-api-key` | `latest` |
| `SUPADATA_API_KEY` | `supadata-api-key` | `latest` |
| `TRANSCRIPTAPI_API_KEY` | `transcriptapi-api-key` | `latest` |

### Optional / Tunable (not set in production by default)

| Variable | Default | Description |
|---|---|---|
| `PLAYLIST_PAGE_SIZE` | `50` | Items per YouTube API page during playlist scan |
| `PLAYLIST_SCAN_THRESHOLD` | `100` | Playlists with ≥ this many entries resume from saved `pageToken` |
| `MAX_PIPELINE_FAILURES` | `3` | Circuit breaker: stop after this many consecutive video failures |

---

## Enabled GCP APIs

Enabled by `infra/setup.sh`:

| API | Service Name |
|---|---|
| Cloud Functions | `cloudfunctions.googleapis.com` |
| Cloud Build | `cloudbuild.googleapis.com` |
| Cloud Run | `run.googleapis.com` |
| Secret Manager | `secretmanager.googleapis.com` |
| Pub/Sub | `pubsub.googleapis.com` |
| Cloud Storage | `storage.googleapis.com` |
| Cloud Scheduler | `cloudscheduler.googleapis.com` |
| YouTube Data API v3 | `youtube.googleapis.com` |

---

## Admin CLI Tools

### `cmd/syncplaylists` — Playlist Sync

**When to run:** Once per year (January) when new year's YouTube playlists are created for each board.

**Prerequisites:** `YOUTUBE_API_KEY` and `MONGODB_URI` set in environment.

```bash
YOUTUBE_API_KEY=$(gcloud secrets versions access latest --secret=youtube-api-key --project=miltonmeetingsummarizer) \
MONGODB_URI=$(gcloud secrets versions access latest --secret=mongodb-uri --project=miltonmeetingsummarizer) \
go run ./cmd/syncplaylists -channelId=UCGnv43oWpciURP-bTDc3GnA
```

**Target playlists** (hardcoded in [cmd/syncplaylists/main.go](../../cmd/syncplaylists/main.go)):
- Select Board 2026
- Planning Board 2026
- Conservation Commission 2026
- School Committee 2026
- Warrant Committee 2026

Update `targetPlaylistTitles` when the year changes.

### `cmd/officials` — Officials Roster Sync

**When to run:** After each election cycle or when committee membership changes. Also safe to run at any time (the collection is completely replaced).

**Prerequisites:** `MONGODB_URI` set. `CHATGPT_API_KEY` only needed if the DOM parser fails.

```bash
MONGODB_URI=$(gcloud secrets versions access latest --secret=mongodb-uri --project=miltonmeetingsummarizer) \
make officials
```

Prints all scraped committees and members to stdout before writing to Firestore. Review output before confirming the database write.

### `cmd/app` — Local Bulk Backfill

**When to run:** Ad-hoc, to process historical videos from a specific playlist. Requires YouTube OAuth credentials (not API key).

**Prerequisites:** `client_secret.json` in repo root, `MONGODB_URI` set.

```bash
MONGODB_URI=... go run ./cmd/app
```

The playlist ID is hardcoded (`PLk5NCS3UGBusb9TkuXtVQhrOhRyezF63S`). Update [cmd/app/main.go](../../cmd/app/main.go) if targeting a different playlist.

---

## Operational Commands

### Deploy the Cloud Function

```bash
make deploy                    # with Facebook posting enabled
make deploy-no-facebook        # with FACEBOOK_ENABLED=false
```

### Test the Webhook Locally

```bash
FUNCTION_URL=$(gcloud functions describe youtube-webhook \
  --region=us-central1 --gen2 --project=miltonmeetingsummarizer \
  --format='value(serviceConfig.uri)')

# Verify hub.challenge response (subscription confirmation)
curl -v "${FUNCTION_URL}?hub.challenge=test-challenge&hub.mode=subscribe"

# Send a test notification
curl -X POST "${FUNCTION_URL}" \
  -H 'Content-Type: application/atom+xml' \
  --data-binary @infra/sample-notification.xml
```

Edit `infra/sample-notification.xml` to set a real video ID before testing.

### Force Subscription Renewal

```bash
gcloud scheduler jobs run renew-youtube-subscription --location=us-central1 --project=miltonmeetingsummarizer
```

### Check Function Logs

```bash
gcloud functions logs read youtube-webhook \
  --region=us-central1 --gen2 --project=miltonmeetingsummarizer \
  --limit=100
```

Or in Cloud Console: Cloud Run → `youtube-webhook` → Logs.

### Check Cloud Build History

```bash
gcloud builds list --filter="trigger_id:<trigger_id>" --project=miltonmeetingsummarizer --limit=10
```

Or in Cloud Console: Cloud Build → History → filter by trigger `hugo-build-and-deploy`.

### Force a Full Site Rebuild

Manually publish a Pub/Sub message to trigger Cloud Build without waiting for a new video:

```bash
gcloud pubsub topics publish youtube-pipeline-trigger \
  --message='{"videoId":"manual"}' \
  --project=miltonmeetingsummarizer
```

### Verify Secret Accessor Permissions

```bash
gcloud secrets get-iam-policy mongodb-uri --project=miltonmeetingsummarizer
```

### Rotate All Secrets (Emergency)

```bash
for SECRET in mongodb-uri openai-api-key facebook-page-id facebook-page-token \
              youtube-api-key supadata-api-key transcriptapi-api-key firebase-ci-token; do
  echo "=== ${SECRET} ==="
  echo -n "new-value: "
  read -rs value && echo
  echo -n "${value}" | gcloud secrets versions add "${SECRET}" \
    --data-file=- --project=miltonmeetingsummarizer
done
# Redeploy to pick up new secret versions
make deploy
```

### Re-run Infra Setup (Idempotent)

```bash
PROJECT_ID=miltonmeetingsummarizer bash infra/setup.sh
```

The setup script uses `|| true` and existence checks throughout, making it safe to re-run if a step failed previously.

---

## Infrastructure Scripts

| Script | Purpose |
|---|---|
| [infra/setup.sh](../../infra/setup.sh) | One-time setup: enable APIs, create SA, set IAM bindings, create GCS bucket, Pub/Sub topic, all secrets, Cloud Build trigger, deploy function, subscribe to PubSubHubbub, create Scheduler job |
| [infra/remaining-setup.sh](../../infra/remaining-setup.sh) | Re-runs the steps that require GitHub to be connected to Cloud Build (Cloud Build trigger creation, function deploy, PubSubHubbub subscription, Scheduler job) |
| [infra/buildtrigger.sh](../../infra/buildtrigger.sh) | Deletes and recreates the `hugo-build-and-deploy` Cloud Build trigger only |
| [infra/sample-notification.xml](../../infra/sample-notification.xml) | Sample Atom/XML payload for manual webhook testing |
| [Makefile](../../Makefile) | `deploy`, `deploy-no-facebook`, `test`, `test-integration`, `officials`, `build-officials` targets |
