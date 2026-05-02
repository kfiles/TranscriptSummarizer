# C4 Level 2 — Containers

This diagram zooms into the Milton Meeting Summarizer system and shows each deployable unit (GCP resource, Firebase service, and admin tooling) as a distinct container. Relationships show data flows and the protocol or mechanism used.

> **Rendering:** Open in VS Code with the PlantUML extension (`Alt+D`), or run `java -jar plantuml.jar docs/architecture/c2-containers.md`.

```plantuml
@startuml C4_Container_Milton_Summarizer
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml

LAYOUT_WITH_LEGEND()

title Container Diagram — Milton Meeting Summarizer

Person(admin, "System Administrator", "Manages playlists, officials,\nand deployment")
Person(resident, "Milton Resident", "Reads meeting summaries")

System_Ext(pshub,       "YouTube PubSubHubbub Hub", "pubsubhubbub.appspot.com")
System_Ext(ytdataapi,   "YouTube Data API v3",      "google.golang.org/api/youtube/v3")
System_Ext(transcapi,   "TranscriptAPI.com",        "Primary transcript provider")
System_Ext(supadata,    "Supadata",                  "Fallback transcript provider")
System_Ext(openai,      "OpenAI API",                "GPT-4.1-mini")
System_Ext(facebook,    "Facebook Graph API v22.0",  "graph.facebook.com/v22.0")
System_Ext(miltonma,    "miltonma.gov",              "Town committee roster page")
System_Ext(github,      "GitHub",                    "Source repository")

System_Boundary(gcp, "GCP Project: miltonmeetingsummarizer") {

  Container(scheduler, "Cloud Scheduler\nrenew-youtube-subscription", "Cloud Scheduler\nSchedule: 0 9 */9 * *", "Sends HTTP POST to PubSubHubbub hub\nevery 9 days to renew the channel\nsubscription before it expires (~10 days)")

  Container(function, "Cloud Function\nyoutube-webhook", "Go 1.24 / Gen 2 Cloud Run\nus-central1 | 512 MiB | 540s timeout\nHTTP trigger (unauthenticated)\nSA: transcript-summarizer", "Entry point for YouTube push notifications.\nRuns the transcription and summarization\npipeline for new meeting videos.\nWrites Hugo Markdown to GCS and\ntriggers site rebuild via Pub/Sub.")

  ContainerDb(firestore, "Firestore Database\nmeetingtranscripts", "Firestore (MongoDB compat)\nMongoDB Wire Protocol API\nCollections: playlists, videos,\ntranscripts, officials", "Operational database. Stores playlist\npagination state, video metadata,\ntranscript text, AI summaries, and\nthe committee officials roster.")

  ContainerQueue(pubsub, "Pub/Sub Topic\nyoutube-pipeline-trigger", "Google Cloud Pub/Sub", "Single-message signal published by the\nCloud Function after uploading new\nMarkdown content to GCS. Triggers\nthe Cloud Build pipeline.")

  Container(gcs, "GCS Bucket\nmiltonmeetingsummarizer-hugo-content", "Google Cloud Storage\nus-central1 | Uniform bucket-level access", "Intermediate content store. Hugo Markdown\nfiles written here by the Cloud Function.\nCloud Build syncs this bucket into its\nworkspace before every Hugo build.\nPath pattern: minutes/YYYY/Month/<videoId>.md")

  Container(cloudbuild, "Cloud Build Trigger\nhugo-build-and-deploy", "Cloud Build (Pub/Sub triggered)\nConfig: cloudbuild.yaml\nbranch: main\nSA: transcript-summarizer", "Pub/Sub-triggered pipeline that builds\nthe Hugo static site and deploys it\nto Firebase Hosting. Steps:\n1. mkdir site/content/minutes\n2. gsutil rsync GCS → workspace\n3. hugo --source=site --minify\n4. npm install firebase-tools\n5. firebase deploy --only hosting")

  Container(secrets, "Cloud Secret Manager", "Secret Manager\n8 secrets (replication: automatic)", "Stores all credentials injected into\nthe Cloud Function at deploy time\nand the firebase-ci-token used\nby Cloud Build.")

  Container(firebase, "Firebase Hosting\nmiltonmeetingsummarizer", "Firebase Hosting (CDN)\nSite: miltonmeetingsummarizer\nPublic dir: site/public", "Static website served at\nmiltonmeetingsummarizer.web.app.\nContent is Hugo-generated HTML.\nPath: /minutes/YYYY/Month/<videoId>/")

  Container(syncplaylists, "Admin Tool\ncmd/syncplaylists", "Go binary (local)\nRequires: YOUTUBE_API_KEY,\nMONGODB_URI", "Fetches playlists from the YouTube\nchannel and upserts them into the\nFirestore playlists collection.\nRun once per year when new\nyear's playlists are created.")

  Container(officials_cmd, "Admin Tool\ncmd/officials", "Go binary (local)\nRequires: MONGODB_URI\n(CHATGPT_API_KEY for LLM fallback)", "Scrapes committee rosters from\nmiltonma.gov and replaces the\nFirestore officials collection.\nRun after election cycles or\nwhen committee membership changes.")
}

' ── Relationships: subscription lifecycle ────────────────────────────────────
Rel(scheduler,  pshub,      "HTTP POST hub.mode=subscribe\n(renews every 9 days)")
Rel(pshub,      function,   "HTTP POST Atom/XML notification\n(new video published)")
Rel(function,   pshub,      "HTTP GET hub.challenge echo\n(subscription confirmation)")

' ── Relationships: pipeline core ─────────────────────────────────────────────
Rel(function,   firestore,  "Read: playlists by channelId\nRead: video (idempotency check)\nWrite: video, transcript records\nMongoDB Wire Protocol / TLS")
Rel(function,   ytdataapi,  "GET playlistItems.list\n(scan playlist for new videos)")
Rel(function,   transcapi,  "GET transcript text by video ID\n(primary; TRANSCRIPTAPI_API_KEY)")
Rel(function,   supadata,   "GET transcript text by video ID\n(fallback; SUPADATA_API_KEY)")
Rel(function,   openai,     "POST transcript + official names\nGET Markdown summary\n(gpt-4.1-mini)")
Rel(function,   gcs,        "PUT minutes/YYYY/Month/<videoId>.md\n(storage.objectCreator role)")
Rel(function,   pubsub,     "Publish JSON build-trigger\n(pubsub.publisher role)")
Rel(function,   facebook,   "POST plain-text summary\nwith transcript URL")
Rel(function,   secrets,    "Reads 7 secrets at startup\nvia Secret Manager injection\n(--set-secrets in deployment)")

' ── Relationships: Cloud Build pipeline ──────────────────────────────────────
Rel(pubsub,     cloudbuild, "Triggers build on message\n(Pub/Sub subscription)")
Rel(cloudbuild, gcs,        "gsutil rsync -r\ngs://bucket/minutes/ → workspace")
Rel(cloudbuild, firebase,   "firebase deploy --only hosting\n(firebasehosting.admin role)")
Rel(cloudbuild, secrets,    "Reads firebase-ci-token\n(secretmanager.secretAccessor)")

' ── Relationships: admin tools ────────────────────────────────────────────────
Rel(syncplaylists, ytdataapi,  "GET playlists.list\n(YOUTUBE_API_KEY)")
Rel(syncplaylists, firestore,  "Upsert playlist records")
Rel(officials_cmd, miltonma,   "HTTP GET Town-Wide page HTML")
Rel(officials_cmd, firestore,  "Drop + insert officials collection\n(BulkWrite not supported;\ndrop-then-insert strategy)")

' ── Relationships: GitHub CI ──────────────────────────────────────────────────
Rel(github,     cloudbuild, "Push to main triggers\ntranscript-summarizer-push\n(separate CI trigger for code)")

' ── Relationships: end user ───────────────────────────────────────────────────
Rel(resident,   firebase,   "HTTPS — reads\nmiltonmeetingsummarizer.web.app")
Rel(admin,      syncplaylists, "Runs annually\n(new year playlists)")
Rel(admin,      officials_cmd, "Runs after elections\n(committee roster updates)")

@enduml
```

---

## Container Reference

### Cloud Function: `youtube-webhook`

| Property | Value |
|---|---|
| Runtime | Go 1.24 |
| Generation | Gen 2 (backed by Cloud Run) |
| Region | `us-central1` |
| Memory | 512 MiB |
| Timeout | 540 seconds |
| HTTP trigger | Unauthenticated (required by YouTube hub) |
| Service account | `transcript-summarizer@<PROJECT_ID>.iam.gserviceaccount.com` |
| Entry point | `YouTubeWebhook` in [function.go](../../function.go) |

The function is both the subscription endpoint (GET for `hub.challenge`) and the pipeline entry point (POST for notifications). Both verbs are handled by the same HTTP handler in [pkg/webhook/handler.go](../../pkg/webhook/handler.go).

---

### Firestore Database: `meetingtranscripts`

Accessed via the MongoDB Wire Protocol compatibility API. The Go application uses the standard `go.mongodb.org/mongo-driver/v2` package with a `MONGODB_URI` connection string. No Firestore-specific SDK is used.

| Collection | Key | Purpose |
|---|---|---|
| `playlists` | `_id = playlistId` | YouTube playlist metadata + pagination state (`pageToken`, `numEntries`) |
| `videos` | `_id = videoId` | Video metadata (title, publishedAt, playlistId). Presence = processed. |
| `transcripts` | `_id = videoId + "_" + languageCode` | Raw transcript text (`retrievedText`) and AI summary (`summaryText`) |
| `officials` | `_id = committeeName` | Committee name and `members: []string` (for summarization name correction) |

**Timeout constraint:** All Firestore operations use a 55-second timeout cap (`capCtx` in [pkg/db/session.go](../../pkg/db/session.go)). Firestore's MongoDB compatibility layer rejects operations with `maxTimeMS > 60000`.

---

### GCS Bucket: `miltonmeetingsummarizer-hugo-content`

| Property | Value |
|---|---|
| Location | `us-central1` |
| Access control | Uniform bucket-level access |
| Object path pattern | `minutes/YYYY/Month/{videoId}.md` |
| Writer | Cloud Function (`roles/storage.objectCreator`) |
| Reader | Cloud Build (`gsutil rsync`) |

The function writes one Markdown file per video. When `WriteAllMarkdown` is called (after any successful run), it re-uploads all videos' Markdown to ensure Cloud Build always has the complete content set, even if previous files were uploaded during a separate function invocation.

---

### Pub/Sub Topic: `youtube-pipeline-trigger`

A signaling mechanism only. The message payload is `{"videoId": "<id>"}` or `{"videoId": "all"}` (from `WriteAllMarkdown`). The Cloud Build trigger does not use the message content — it uses the message as a signal to start a build that syncs from GCS.

---

### Cloud Build: `hugo-build-and-deploy`

| Property | Value |
|---|---|
| Trigger type | Pub/Sub |
| Topic | `projects/<PROJECT_ID>/topics/youtube-pipeline-trigger` |
| Source | GitHub repo, `main` branch |
| Build config | `cloudbuild.yaml` |
| Service account | `transcript-summarizer` SA (for logging) |
| Logging | `CLOUD_LOGGING_ONLY` |

Substitutions passed at trigger creation:
- `_CONTENT_BUCKET` — GCS bucket name
- `_FIREBASE_PROJECT` — `miltonmeetingsummarizer`

The `firebase-ci-token` secret is read by Cloud Build for the `firebase deploy` step. It is injected into the build environment by the Firebase CLI (not as an env var). The Cloud Build SA needs `roles/secretmanager.secretAccessor` on the `firebase-ci-token` secret.

---

### Firebase Hosting: `miltonmeetingsummarizer`

| Property | Value |
|---|---|
| Site ID | `miltonmeetingsummarizer` |
| Public directory | `site/public` |
| URL | `https://miltonmeetingsummarizer.web.app` |
| Hugo theme | `hugo-geekdoc` |
| Base URL | `https://miltonmeetingsummarizer.web.app` |

Hugo's content structure maps directly to URL paths: `site/content/minutes/2025/January/abc123.md` becomes `/minutes/2025/January/abc123/`.

---

### Cloud Scheduler: `renew-youtube-subscription`

| Property | Value |
|---|---|
| Schedule | `0 9 */9 * *` (every 9 days at 09:00 UTC) |
| Method | `POST` to `https://pubsubhubbub.appspot.com/subscribe` |
| Body | `hub.callback=<functionURL>&hub.topic=<feedURL>&hub.mode=subscribe&hub.verify=async` |
| Rationale | YouTube subscriptions expire after ~10 days; 9-day renewal ensures continuity |

The Scheduler calls the hub directly — the Cloud Function is not involved in the renewal request itself. The function only needs to respond to the subsequent `hub.challenge` GET from the hub to confirm the renewed subscription.

---

### Cloud Secret Manager

| Secret | Injected as | Consumed by |
|---|---|---|
| `mongodb-uri` | `MONGODB_URI` | Cloud Function, Cloud Build (integration tests) |
| `openai-api-key` | `CHATGPT_API_KEY` | Cloud Function |
| `facebook-page-id` | `FACEBOOK_PAGE_ID` | Cloud Function |
| `facebook-page-token` | `FACEBOOK_PAGE_TOKEN` | Cloud Function |
| `youtube-api-key` | `YOUTUBE_API_KEY` | Cloud Function |
| `supadata-api-key` | `SUPADATA_API_KEY` | Cloud Function |
| `transcriptapi-api-key` | `TRANSCRIPTAPI_API_KEY` | Cloud Function |
| `firebase-ci-token` | *(firebase CLI reads directly)* | Cloud Build |

Secrets are injected via `--set-secrets` at deploy time, not pulled at runtime. The function process sees them as environment variables from the moment it starts.

---

### Admin Tools

Both tools are local Go binaries in the `cmd/` directory. They share the same Firestore connection logic as the Cloud Function (`db.NewClient()` reads `MONGODB_URI`) but run from a developer's machine.

**`cmd/syncplaylists`** — Populates the `playlists` collection. Must be run once per year when YouTube channel playlists roll over to a new year. The list of target playlist titles is hardcoded in `targetPlaylistTitles` in [cmd/syncplaylists/main.go](../../cmd/syncplaylists/main.go).

**`cmd/officials`** — Refreshes the `officials` collection. Uses `ParseTownWideDOM` by default (deterministic HTML parser). If the miltonma.gov page structure changes and the DOM parser stops working, `LLMExtractor.ParseTownWideLLM` is available as a fallback (requires `CHATGPT_API_KEY`).
