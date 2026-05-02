# Architecture Overview — Milton Meeting Summarizer

## Purpose

Milton Meeting Summarizer is an automated pipeline that monitors the Milton, MA town government's YouTube channel for new meeting recordings, transcribes them using a third-party API, generates unofficial meeting minutes using GPT-4.1-mini, publishes those minutes to a Hugo-based static website hosted on Firebase, and posts a summary to Facebook.

The system is event-driven and fully serverless. A new meeting video being published to YouTube is the only external trigger required for the full end-to-end pipeline to run.

---

## High-Level Architecture

```
YouTube Channel
      |
      | new video published (WebSub/PubSubHubbub)
      v
Cloud Function (youtube-webhook)
      |
      |-- query Firestore for playlists
      |-- scan YouTube Data API for new videos
      |-- fetch transcript (TranscriptAPI.com / Supadata)
      |-- summarize via OpenAI GPT-4.1-mini
      |-- store transcript + summary in Firestore
      |-- write Hugo markdown → /tmp
      |-- upload markdown → GCS bucket
      |-- publish message → Pub/Sub topic
      |-- post summary → Facebook Page
      |
      v
Cloud Build trigger (hugo-build-and-deploy)
      |
      |-- gsutil rsync GCS → site/content/minutes/
      |-- hugo build --minify
      |-- firebase deploy → Firebase Hosting
      |
      v
miltonmeetingsummarizer.web.app
```

---

## System Components

### Cloud Function: `youtube-webhook`

The central piece of the system. A Gen 2 Cloud Function (backed by Cloud Run) that:

- Receives HTTP push notifications from YouTube's PubSubHubbub hub when a new video is published to the channel.
- Responds to YouTube's `hub.challenge` GET requests to confirm subscription validity.
- On POST: parses the Atom XML payload, extracts `channelId`, and runs the video processing pipeline.

The function is deployed with `--allow-unauthenticated` because the YouTube hub cannot include Google auth headers. YouTube sends push notifications from its own infrastructure without any authentication token.

### Firestore (MongoDB Compatibility Mode): `meetingtranscripts`

The operational database. Firestore is accessed via its **MongoDB Wire Protocol compatibility API**, which means the application uses the standard Go MongoDB driver (`go.mongodb.org/mongo-driver/v2`) and a `MONGODB_URI` connection string — no Firebase or Firestore SDK is used for data access.

Collections:
- **`playlists`** — YouTube playlist metadata including pagination state (`pageToken`, `numEntries`) that allows the pipeline to scan from where it left off rather than re-scanning from the beginning on every webhook call.
- **`videos`** — Metadata for each processed video (ID, title, publish date, playlist). A video's presence in this collection is the idempotency guard: the pipeline skips any video already recorded here.
- **`transcripts`** — Raw transcript text and AI-generated summary Markdown, keyed by `videoId + "_" + languageCode`.
- **`officials`** — Committee roster data scraped from miltonma.gov. Used during summarization to ensure official names are spelled correctly in the AI output.

> **Note on operation timeout:** Firestore's MongoDB compatibility layer rejects `maxTimeMS` values above 60,000 ms. All database calls are wrapped with a `capCtx()` helper that enforces a 55-second timeout regardless of the parent context's deadline. See [pkg/db/session.go](../../pkg/db/session.go).

### GCS Bucket: `miltonmeetingsummarizer-hugo-content`

An intermediate content store. The Cloud Function writes Hugo-formatted Markdown files to a local `/tmp` directory during execution and then uploads them to this bucket. The Cloud Build pipeline then `rsync`s from this bucket into its workspace before running Hugo. This decouples content generation (done by the function, which may process multiple videos) from the site build (triggered once after all content is written).

### Pub/Sub Topic: `youtube-pipeline-trigger`

A single-message trigger. After the function uploads new content to GCS, it publishes a message `{"videoId": "<id>"}` to this topic. The Cloud Build trigger listens on this topic and starts a build job. No queue semantics are needed; the message is purely a signal.

### Cloud Build: `hugo-build-and-deploy`

A Pub/Sub-triggered build pipeline defined in [cloudbuild.yaml](../../cloudbuild.yaml). Steps:

1. `busybox mkdir` — create `site/content/minutes/`
2. `gsutil rsync` — sync all Markdown files from GCS into the workspace
3. `hugo --source=site --minify` — build the static site
4. `npm install firebase-tools` — install the Firebase CLI locally
5. `firebase deploy --only hosting` — push the built site to Firebase Hosting

The function's service account (`transcript-summarizer`) acts as the build runner, which is why it holds `roles/logging.logWriter`.

### Firebase Hosting: `miltonmeetingsummarizer`

The public website at `https://miltonmeetingsummarizer.web.app`. A Hugo static site using the `hugo-geekdoc` theme, organized as `minutes/YYYY/Month/<videoId>/`. Content comes entirely from Cloud Build deploys triggered by the Pub/Sub pipeline.

### Cloud Scheduler: `renew-youtube-subscription`

YouTube PubSubHubbub subscriptions expire after approximately 10 days. This job fires every 9 days (`0 9 */9 * *`) and sends a re-subscribe `POST` directly to `pubsubhubbub.appspot.com`. The function does not need to be involved in renewals — the Scheduler calls the hub directly. The subscription confirmation `GET` (hub.challenge) is handled by the function.

### Cloud Secret Manager

All credentials are stored as Secret Manager secrets and injected into the Cloud Function at deploy time via `--set-secrets`. They appear as environment variables inside the function process. See [cloud-resources.md](./cloud-resources.md) for the full mapping.

---

## Admin CLI Tools

Three local command-line programs live in [cmd/](../../cmd/). They are not deployed; they run from a developer's machine with `MONGODB_URI` set.

| Tool | Command | Purpose | When to run |
|---|---|---|---|
| `cmd/syncplaylists` | `go run ./cmd/syncplaylists -channelId=UCGnv43oWpciURP-bTDc3GnA` | Fetches playlists from YouTube and upserts them into Firestore. Filters to `targetPlaylistTitles` (hardcoded list of boards). | Once per year when new year's playlists are created on YouTube. |
| `cmd/officials` | `make officials` | Scrapes committee rosters from miltonma.gov and replaces the `officials` collection in Firestore. | When committee membership changes or after each election cycle. |
| `cmd/app` | `go run ./cmd/app` | Local bulk processor: fetches all items from a hardcoded playlist and runs the full pipeline for each video. Uses YouTube OAuth (requires `client_secret.json`). | Ad-hoc, for bulk backfill of historical videos. |

---

## Transcript Provider Strategy

The pipeline uses a pluggable transcript provider controlled by the `TRANSCRIPT_PROVIDER` environment variable. The factory is in [pkg/transcript/transcriber.go](../../pkg/transcript/transcriber.go):

| `TRANSCRIPT_PROVIDER` | Provider | Auth |
|---|---|---|
| `transcriptapi` (current default) | TranscriptAPI.com | Bearer token (`TRANSCRIPTAPI_API_KEY`) |
| `supadata` (or empty) | Supadata | API key (`SUPADATA_API_KEY`) |
| `youtube` | Direct YouTube caption scrape | None (public data) |

The providers are fully independent implementations behind the `VideoTranscriber` interface. Switching providers requires only a change to the `TRANSCRIPT_PROVIDER` env var in the function deployment (no code changes).

---

## Key Design Decisions

### Idempotency via Firestore presence check
Before processing any video, the pipeline calls `facade.GetVideo()`. If the video already exists in Firestore, it is skipped entirely. This means re-runs and duplicate hub notifications are safely ignored.

### Playlist page token tracking
Each playlist record stores `pageToken` (the last YouTube page token successfully scanned) and `numEntries` (how many items were in the playlist at that time). When a webhook fires, the pipeline starts scanning from `pageToken` rather than from the beginning — critical for playlists with hundreds of videos.

### Circuit breaker
The pipeline tolerates up to `MAX_PIPELINE_FAILURES` (default: 3) consecutive video failures before stopping processing for a given webhook invocation. This prevents a single corrupted video from blocking all subsequent videos in the same scan, while also avoiding runaway API calls if all transcripts are unavailable.

### GCS as content staging
The Cloud Function writes Markdown to `/tmp` and uploads to GCS. Cloud Build then syncs the entire bucket into its workspace. This means Hugo always builds with the complete content catalogue — not just the file from the triggering video.

### Officials: drop-then-insert
The `cmd/officials` tool drops the entire `officials` collection and re-inserts all committees on each run, instead of upsert-per-document. This is because Firestore's MongoDB compatibility layer does not support `BulkWrite`. The entire refresh takes under one second.

### IAM conditional binding for Firestore
The `transcript-summarizer` service account's `roles/datastore.user` is bound with a condition scoped to `projects/.../databases/meetingtranscripts` specifically, because the project's IAM policy requires an explicit condition on all `datastore.user` bindings. This is noted in [infra/setup.sh](../../infra/setup.sh).

---

## Documentation Map

| Document | Content |
|---|---|
| [c1-context.md](./c1-context.md) | C4 Level 1 — System context: actors, the system as a black box, external dependencies |
| [c2-containers.md](./c2-containers.md) | C4 Level 2 — All GCP/Firebase containers and their relationships |
| [c3-components.md](./c3-components.md) | C4 Level 3 — Go packages inside the Cloud Function |
| [c4-code.md](./c4-code.md) | C4 Level 4 — Sequence diagrams for each code path |
| [cloud-resources.md](./cloud-resources.md) | Full inventory: GCP resources, IAM, secrets, env vars, operational commands |

> **Rendering PlantUML diagrams:** The C4 diagrams in c1–c3 use PlantUML syntax. Render them with the [VS Code PlantUML extension](https://marketplace.visualstudio.com/items?itemName=jebbs.plantuml), IntelliJ's built-in PlantUML support, or `java -jar plantuml.jar <file>`. The c4-code.md sequence diagrams use Mermaid and render natively in GitHub and VS Code.
