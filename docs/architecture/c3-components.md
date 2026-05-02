# C4 Level 3 — Components

This diagram zooms into the Cloud Function container and shows its internal Go packages (components), their responsibilities, and how they interact with each other and with external systems. Admin CLI tools share several packages with the Cloud Function and are shown separately.

> **Rendering:** Open in VS Code with the PlantUML extension (`Alt+D`), or run `java -jar plantuml.jar docs/architecture/c3-components.md`.

```plantuml
@startuml C4_Component_Milton_Summarizer
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Component.puml

LAYOUT_WITH_LEGEND()

title Component Diagram — Cloud Function (youtube-webhook) and Admin Tools

' ── External containers ───────────────────────────────────────────────────────
ContainerDb(firestore,   "Firestore\nmeetingtranscripts",   "MongoDB compat", "")
Container(gcs,           "GCS Bucket",                       "Cloud Storage",  "")
ContainerQueue(pubsub,   "Pub/Sub Topic",                    "Cloud Pub/Sub",  "")
System_Ext(pshub,        "PubSubHubbub Hub",                 "")
System_Ext(ytdataapi,    "YouTube Data API v3",              "")
System_Ext(transcapi,    "TranscriptAPI.com",                "")
System_Ext(supadata_ext, "Supadata",                         "")
System_Ext(openai_ext,   "OpenAI API",                       "")
System_Ext(facebook_ext, "Facebook Graph API",               "")
System_Ext(miltonma,     "miltonma.gov",                     "")

' ── Cloud Function boundary ───────────────────────────────────────────────────
Container_Boundary(fn, "Cloud Function: youtube-webhook (Go 1.24)") {

  Component(entry, "function.go\nYouTubeWebhook()", "Go package: transcriptsummarizer\n(root package)", "Cloud Functions buildpack entry point.\nForwards to webhook.Handler.\nExported symbol required by runtime.")

  Component(webhook, "pkg/webhook\nhandler.go", "HTTP handler\nAtom/XML parser", "Handles GET (hub.challenge echo for\nsubscription confirmation) and POST\n(new video notification). Parses Atom\nXML payload to extract channelId.\nOrchestrates the pipeline loop:\n- Lists playlists from Firestore\n- Scans playlist items from YouTube\n- Skips already-processed videos\n- Runs pipeline.Run per new video\n- Calls pipeline.WriteAllMarkdown\n  if any video succeeded\nCircuit breaker: stops after\nMAX_PIPELINE_FAILURES (default 3)\nconsecutive failures.")

  Component(pipeline, "pkg/pipeline\npipeline.go", "Pipeline orchestrator\nGCS uploader\nPub/Sub publisher", "Run(ctx, facade, client, video):\n  Transcribes → summarizes →\n  stores → writes Markdown →\n  uploads to GCS → publishes\n  Pub/Sub signal → posts to Facebook.\n\nWriteAllMarkdown(ctx, facade, client):\n  Re-renders Markdown for all videos\n  with summaries and uploads to GCS.\n  Called after every successful run\n  to keep GCS complete.\n\nuploadToGCS(): Cloud Storage client.\npublishBuildTrigger(): Pub/Sub client.")

  Component(webhook_iface, "pkg/transcript\ntranscriber.go", "VideoTranscriber interface\nProvider factory", "Defines the VideoTranscriber interface:\n  Transcribe(ctx, videoID) (text, lang, error)\n\nNewVideoTranscriber() selects provider\nfrom TRANSCRIPT_PROVIDER env var:\n  'transcriptapi' → TranscriptAPITranscriber\n  'supadata'      → SupadataTranscriber\n  ''  (default)   → SupadataTranscriber\n  'youtube'       → YouTubeTranscriber")

  Component(transcriptapi_impl, "pkg/transcript\ntranscriptapi.go", "TranscriptAPITranscriber\n(primary provider)", "Calls transcriptapi.com/api/v2.\nBearer token auth (TRANSCRIPTAPI_API_KEY).\nRetries up to 3 times on 408/429/503.\nReturns ErrTranscriptUnavailable on 404.")

  Component(supadata_impl, "pkg/transcript\nsupadata.go", "SupadataTranscriber\n(fallback provider)", "Calls api.supadata.ai/v1.\nx-api-key header auth (SUPADATA_API_KEY).\nHandles 200 (sync) and 202 (async job).\nFor async: polls at 1s intervals until\nstatus=completed or failed.")

  Component(youtube_impl, "pkg/transcript\nyoutube_transcriber.go\ntranscript.go", "YouTubeTranscriber\n(last-resort provider)", "Fetches the YouTube watch page HTML,\nextracts ytInitialPlayerResponse JSON,\nand downloads caption track XML directly.\nNo API key required. Fragile; depends\non YouTube's internal page structure.")

  Component(playlist_pkg, "pkg/transcript\nplaylist.go", "Playlist and Video types\nYouTube API helpers", "Defines Playlist and Video structs.\nScanPlaylist(): paginates through\n  playlist items using API key.\n  Tracks page tokens for resumption.\nGetChannelPlaylists(): lists all\n  playlists for a channel ID.\nGetVideoByID(): fetches single video\n  metadata by video ID.")

  Component(summarize, "pkg/summarize\nopenai.go", "OpenAI client\nPrompt builder", "Calls OpenAI chat completions API.\nModel: gpt-4.1-mini.\nSystem prompt: professional town\nreporter formatting unofficial minutes.\nUser prompt: base summary request +\n  optional 'spell these names correctly'\n  appendix when officials list is provided.\nAuth: CHATGPT_API_KEY env var.")

  Component(facebook_pkg, "pkg/facebook\nfacebook.go", "Facebook Graph API client\nMarkdown → plain-text renderer", "PostToPage(): POST to\n  graph.facebook.com/v22.0/<pageId>/feed.\nFormatPost(): converts Markdown summary\n  to Facebook-friendly plain text using\n  a custom ast.Renderer (headings become\n  ━━━ HEADING ━━━, lists use bullets).\nTranscriptURL(): builds the website URL\n  for the video transcript page.\nGated by FACEBOOK_ENABLED env var.")

  Component(db_facade, "pkg/db\nfacade.go + session.go", "db.Facade interface\nMongoDB client factory", "Facade interface: 17 methods covering\nCRUD for playlists, videos, transcripts.\nNewFacade() returns the production impl.\nNewClient() reads MONGODB_URI and\nreturns a *mongo.Client.\ncapCtx(): enforces 55s max timeout\non all Firestore operations\n(Firestore rejects > 60s maxTimeMS).\nDatabase name: meetingtranscripts.")

  Component(playlist_store, "pkg/db\nplaylist_store.go", "PlaylistStore\nCollection: playlists", "ListPlaylists(channelId)\nGetPlaylist(playlistID)\nInsertPlaylist / UpdatePlaylist\nUpsertPlaylist / DeletePlaylist\nUpdatePlaylist sets pageToken,\nnumEntries, updatedAt.")

  Component(video_store, "pkg/db\nvideo_store.go", "VideoStore\nCollection: videos", "ListAllVideos()\nGetVideo(videoID) — used as\n  idempotency guard: skip if found.\nInsertVideo()\nUpdateVideo / DeleteVideo\n(stubs — not yet implemented)")

  Component(transcript_store, "pkg/db\ntranscript_store.go", "TranscriptStore\nCollection: transcripts", "ListTranscripts(videoId)\nGetTranscript(videoID, languageCode)\nInsertTranscript()\nUpdateTranscript() — updates\n  summaryText field only.\nDeleteTranscript() (stub)")

  Component(officials_store, "pkg/db\nofficials_store.go", "OfficialsStore\nCollection: officials", "ListOfficialNames(ctx, client)\n  Returns a flat slice of all member names\n  across all committee documents.\n  Soft failure: returns nil if empty,\n  pipeline continues without names.")

}

' ── Admin CLI boundary ────────────────────────────────────────────────────────
Container_Boundary(admin_tools, "Admin CLI Tools (local binaries)") {

  Component(sync_main, "cmd/syncplaylists\nmain.go", "Playlist sync tool\nRequires: YOUTUBE_API_KEY,\nMONGODB_URI", "Fetches all playlists for the channel\nfrom YouTube, filters to\ntargetPlaylistTitles (hardcoded list),\nand upserts each into Firestore.\nUses playlist_pkg.GetChannelPlaylists()\nand db_facade.UpsertPlaylist().")

  Component(app_main, "cmd/app\nmain.go", "Bulk backfill tool\nRequires: OAuth client_secret.json,\nMONGODB_URI", "Fetches all items from a hardcoded\nplaylist (PLk5NCS3UGBusb9TkuXtVQhrOhRyezF63S)\nusing YouTube OAuth, then calls\npipeline.Run() for each video.\nFor local/historical backfill only.")

  Component(officials_main, "cmd/officials\nmain.go", "Officials roster tool\nRequires: MONGODB_URI\n(CHATGPT_API_KEY for LLM fallback)", "Scrapes miltonma.gov Town-Wide page.\nCalls officials.ParseTownWideDOM()\n(deterministic; targets CivicPlus\nCSS classes).\nDrops and re-inserts the officials\ncollection in Firestore.\nLLMExtractor.ParseTownWideLLM()\navailable as fallback if DOM\nparser breaks.")

  Component(officials_pkg, "pkg/officials\nofficials.go + dom.go + llm.go", "Town officials scraper\nDOM and LLM strategies", "Fetch(): HTTP GET miltonma.gov/890/Town-Wide.\nParseTownWideDOM(): deterministic HTML\n  parser using golang.org/x/net/html.\n  Targets .tabbedWidget.cpTabPanel divs\n  and .widgetTitle.field.p-name h4 nodes.\n  Falls back to .subhead2 h3 for sections\n  that don't use the CityDirectory widget.\nnormalizeCommitteeName(): title-cases\n  committee names, handles small words.\nLLMExtractor.ParseTownWideLLM():\n  Sends full HTML to gpt-4.1-mini,\n  returns JSON list of committees+members.\n  Used only as manual fallback.")

}

' ── Internal relationships: entry → webhook → pipeline ───────────────────────
Rel(entry,             webhook,           "delegates to\nwebhook.Handler()")
Rel(webhook,           pipeline,          "calls pipeline.Run()\nper new video;\ncalls pipeline.WriteAllMarkdown()\nafter successful runs")
Rel(webhook,           db_facade,         "NewFacade() + NewClient()\nListPlaylists()\nGetVideo() (idempotency check)")
Rel(webhook,           playlist_pkg,      "ScanPlaylist(playlistId,\npageToken, pageSize)")

' ── Pipeline → sub-packages ──────────────────────────────────────────────────
Rel(pipeline,          webhook_iface,     "NewVideoTranscriber()\n.Transcribe(ctx, videoID)")
Rel(pipeline,          summarize,         "Summarize(ctx, text, names)")
Rel(pipeline,          db_facade,         "InsertVideo()\nInsertTranscript()\nUpdateTranscript()\nListAllVideos()\nListTranscripts()")
Rel(pipeline,          officials_store,   "ListOfficialNames()\n(via db.ListOfficialNames)")
Rel(pipeline,          facebook_pkg,      "FormatPost()\nPostToPage()\nTranscriptURL()")

' ── Transcript provider selection ─────────────────────────────────────────────
Rel(webhook_iface,     transcriptapi_impl, "TRANSCRIPT_PROVIDER\n= 'transcriptapi'")
Rel(webhook_iface,     supadata_impl,      "TRANSCRIPT_PROVIDER\n= 'supadata' or ''")
Rel(webhook_iface,     youtube_impl,       "TRANSCRIPT_PROVIDER\n= 'youtube'")

' ── db facade → stores ───────────────────────────────────────────────────────
Rel(db_facade,         playlist_store,     "collection: playlists")
Rel(db_facade,         video_store,        "collection: videos")
Rel(db_facade,         transcript_store,   "collection: transcripts")
Rel(db_facade,         officials_store,    "collection: officials")

' ── Stores → Firestore ────────────────────────────────────────────────────────
Rel(playlist_store,    firestore,          "CRUD via MongoDB driver")
Rel(video_store,       firestore,          "CRUD via MongoDB driver")
Rel(transcript_store,  firestore,          "CRUD via MongoDB driver")
Rel(officials_store,   firestore,          "Read via MongoDB driver")

' ── External calls ────────────────────────────────────────────────────────────
Rel(playlist_pkg,      ytdataapi,          "GET playlistItems.list\n(API key)")
Rel(transcriptapi_impl, transcapi,         "GET transcript")
Rel(supadata_impl,     supadata_ext,       "GET transcript\n(sync or async poll)")
Rel(youtube_impl,      ytdataapi,          "GET youtube.com/watch HTML\n(no auth)")
Rel(summarize,         openai_ext,         "POST chat/completions\n(gpt-4.1-mini)")
Rel(facebook_pkg,      facebook_ext,       "POST feed\n(page token)")
Rel(pipeline,          gcs,                "PUT object\n(cloud.google.com/go/storage)")
Rel(pipeline,          pubsub,             "Publish message\n(cloud.google.com/go/pubsub/v2)")

' ── Admin tools → shared packages ────────────────────────────────────────────
Rel(sync_main,         playlist_pkg,       "GetChannelPlaylists()")
Rel(sync_main,         db_facade,          "UpsertPlaylist()")
Rel(sync_main,         firestore,          "via db_facade")
Rel(app_main,          pipeline,           "pipeline.Run()")
Rel(app_main,          db_facade,          "NewClient() + NewFacade()")
Rel(officials_main,    officials_pkg,      "Fetch() + ParseTownWideDOM()")
Rel(officials_main,    firestore,          "Drop + InsertMany\n(officials collection)")
Rel(officials_pkg,     miltonma,           "HTTP GET /890/Town-Wide")
Rel(officials_pkg,     openai_ext,         "LLM fallback:\nParseTownWideLLM()")

' ── Subscription confirmation ─────────────────────────────────────────────────
Rel(webhook,           pshub,              "GET hub.challenge echo\n(subscription confirm)")

@enduml
```

---

## Package Reference

### `function.go` — Entry Point

**File:** [function.go](../../function.go)

The Cloud Functions buildpack requires an exported Go function at the package level. This file is the only file in the root package; it imports and delegates to `webhook.Handler`. The buildpack discovers `YouTubeWebhook` by name at startup.

---

### `pkg/webhook` — HTTP Handler and Pipeline Orchestrator

**Files:** [pkg/webhook/handler.go](../../pkg/webhook/handler.go)

Responsibilities:
- Route `GET` (subscription challenge) vs `POST` (notification) requests.
- Parse Atom/XML: extract `channelId` from `<yt:channelId>` and `videoId` from `<yt:videoId>`.
- For each playlist associated with the channel: call `transcript.ScanPlaylist` to list playlist items from YouTube, compare against videos already in Firestore, and run `pipeline.Run` for each new one.
- Maintain a circuit breaker: `failCount >= MAX_PIPELINE_FAILURES` (default 3) aborts further processing for this invocation.
- After any successful video: call `pipeline.WriteAllMarkdown` to ensure GCS is up to date.
- Always return `204 No Content` to YouTube, even on pipeline error — YouTube must not retry.

**Environment variables read:**
- `PLAYLIST_PAGE_SIZE` (default: 50) — items per YouTube API page
- `PLAYLIST_SCAN_THRESHOLD` (default: 100) — playlists with ≥ this many entries use the stored `pageToken` as start position
- `MAX_PIPELINE_FAILURES` (default: 3) — circuit breaker threshold

**All function variables are package-level vars** (`runPipelineFn`, `newFacadeFn`, etc.) so tests can inject stubs without a running server.

---

### `pkg/pipeline` — Per-Video Processing Pipeline

**Files:** [pkg/pipeline/pipeline.go](../../pkg/pipeline/pipeline.go)

`Run(ctx, facade, client, video)` is the core processing unit for a single video:

1. Check if video exists in Firestore (idempotency guard via `GetVideo`).
2. Call `newTranscriber().Transcribe(ctx, videoID)` — provider selected by `TRANSCRIPT_PROVIDER`.
3. Handle `ErrTranscriptUnavailable`: record the video with description "Transcript unavailable" and return without error. This ensures the video is not retried indefinitely.
4. Fetch official names via `db.ListOfficialNames` (soft failure — continues without names if the collection is empty).
5. Check for existing transcript in Firestore; if missing, insert raw text and call `summarize.Summarize`.
6. Update transcript record with `summaryText`.
7. Write Hugo Markdown to `HUGO_CONTENT_DIR` (default `/tmp/hugo-content/minutes`).
8. Upload Markdown to GCS (`uploadToGCS`).
9. Publish Pub/Sub message (`publishBuildTrigger`).
10. Post to Facebook if `FACEBOOK_ENABLED != false` and credentials are set.

`WriteAllMarkdown(ctx, facade, client)` regenerates and re-uploads Markdown for every video with a non-empty summary. Called after any successful pipeline run to keep GCS complete.

**Hugo Markdown format:**
```toml
+++
title = 'Video Title'
date = 2025-01-15T19:00:00-05:00
draft = false
+++
{summaryText}
```

**GCS object path:** `minutes/{YYYY}/{Month}/{videoId}.md`

---

### `pkg/transcript` — Transcription Providers

**Files:** [pkg/transcript/transcriber.go](../../pkg/transcript/transcriber.go), [transcriptapi.go](../../pkg/transcript/transcriptapi.go), [supadata.go](../../pkg/transcript/supadata.go), [youtube_transcriber.go](../../pkg/transcript/youtube_transcriber.go)

#### `VideoTranscriber` interface
```go
type VideoTranscriber interface {
    Transcribe(ctx context.Context, videoID string) (text string, languageCode string, err error)
}
```

All three providers implement this interface. `NewVideoTranscriber()` is the factory function — it reads `TRANSCRIPT_PROVIDER` and returns the appropriate implementation.

#### `ErrTranscriptUnavailable`
Sentinel error defined in [errors.go](../../pkg/transcript/errors.go). All providers return this (wrapped) when the transcript simply doesn't exist for a video. The pipeline handles it specially: record the video as "unavailable" and skip without error.

#### `TranscriptAPITranscriber`
- Synchronous GET to `https://transcriptapi.com/api/v2/youtube/transcript`
- Retries up to `maxTranscriptAPIRetries` (3) times on 408, 429, or 503
- Respects `Retry-After` header on 429
- Returns `ErrTranscriptUnavailable` on 404

#### `SupadataTranscriber`
- GET to `https://api.supadata.ai/v1/transcript`
- 200 → synchronous result (returns immediately)
- 202 → async: polls `GET /v1/transcript/{jobId}` at 1-second intervals until `status` is `completed` or `failed`
- Returns `ErrTranscriptUnavailable` on 404

#### `YouTubeTranscriber`
- Fetches `https://www.youtube.com/watch?v={videoID}` (raw HTML)
- Extracts `ytInitialPlayerResponse` JSON from the page
- Parses caption track list and downloads the first available track as XML
- No API key required; brittle — depends on YouTube's internal page structure

---

### `pkg/transcript` — Playlist and Video Helpers

**Files:** [pkg/transcript/playlist.go](../../pkg/transcript/playlist.go)

These functions interact with the YouTube Data API:

- **`ScanPlaylist(playlistId, startPageToken, pageSize)`** — Paginates through playlist items starting from `startPageToken`. Returns `[]*PlaylistEntry` sorted by `PublishedAt` ascending. Each entry carries the `PageToken` used to retrieve its page (for resumption tracking). Uses `YOUTUBE_API_KEY`.
- **`GetChannelPlaylists(channelId)`** — Lists all playlists for a channel. Used by `cmd/syncplaylists`. Uses `YOUTUBE_API_KEY`.
- **`GetVideoByID(videoID)`** — Fetches single video metadata. Uses `YOUTUBE_API_KEY`.
- **`GetPlaylists()` / `GetPlaylistItems()`** — OAuth-based variants; used only by `cmd/app` with `client_secret.json`.

---

### `pkg/summarize` — OpenAI Summarizer

**File:** [pkg/summarize/openai.go](../../pkg/summarize/openai.go)

Sends the raw transcript text to `gpt-4.1-mini` with a system prompt that establishes the model as a professional town reporter. If an `[]string` of official names is provided, they are appended to the user prompt as a spelling correction instruction.

Output is Markdown formatted as unofficial meeting minutes, organized by discussion topic. The text is stored as `summaryText` in the `transcripts` collection and embedded directly in Hugo Markdown files.

---

### `pkg/facebook` — Facebook Publisher

**File:** [pkg/facebook/facebook.go](../../pkg/facebook/facebook.go)

- **`FormatPost(title, markdownSummary, transcriptURL)`** — Converts the Markdown summary to Facebook-friendly plain text using a custom `ast.Renderer`. Headings become `━━━ HEADING ━━━`, list items use `•` bullets. The `transcriptURL` (if set) is appended as a plain URL for Facebook's auto-link feature.
- **`PostToPage(pageID, token, message)`** — `POST https://graph.facebook.com/v22.0/{pageId}/feed` with `message` and `access_token` form fields.
- **`TranscriptURL(projectID, videoID, publishedAt)`** — Builds `https://{projectID}.web.app/minutes/YYYY/Month/{videoID}/`.

Facebook posting is gated by `FACEBOOK_ENABLED != "false"` AND `FACEBOOK_PAGE_ID != ""` AND `FACEBOOK_PAGE_TOKEN != ""`.

---

### `pkg/db` — Database Facade

**Files:** [pkg/db/facade.go](../../pkg/db/facade.go), [session.go](../../pkg/db/session.go), [playlist_store.go](../../pkg/db/playlist_store.go), [video_store.go](../../pkg/db/video_store.go), [transcript_store.go](../../pkg/db/transcript_store.go), [officials_store.go](../../pkg/db/officials_store.go)

The `Facade` interface abstracts all Firestore access. The production implementation (`dbFacade`) uses the MongoDB Go driver. Tests inject a stub via `newFacadeFn` in `pkg/webhook`.

All store methods call `capCtx(parent)` to enforce a 55-second deadline on every Firestore operation. This is a hard constraint from Firestore's MongoDB compatibility layer.

**Database name constant:** `meetingtranscripts` (in `session.go`)

---

### `pkg/officials` — Committee Roster Scraper

**Files:** [pkg/officials/officials.go](../../pkg/officials/officials.go), [dom.go](../../pkg/officials/dom.go), [llm.go](../../pkg/officials/llm.go)

Used exclusively by `cmd/officials`. Not imported by the Cloud Function.

- **`Fetch(ctx, client)`** — HTTP GET of the Town-Wide page with a browser-like User-Agent header.
- **`ParseTownWideDOM(r io.Reader)`** — Walks the HTML tree targeting CivicPlus widget CSS classes (`.tabbedWidget.cpTabPanel`, `.widgetTitle.field.p-name`). Falls back to `.subhead2` for sections using a different widget layout. Deterministic and zero-cost.
- **`LLMExtractor.ParseTownWideLLM(ctx, htmlBody)`** — Sends the full HTML to `gpt-4.1-mini` asking for `{"committees": [{"name": string, "members": [string]}]}`. Used only as a manual fallback when the DOM parser breaks due to page structure changes.
- **`normalizeCommitteeName(s)`** — Title-cases committee names with small-word rules (of, the, and, for, etc.).

---

### `pkg/render` — Plain-text Renderer

**File:** [pkg/render/text_renderer.go](../../pkg/render/text_renderer.go)

A Markdown-to-plain-text renderer with sanitization rules: replaces "Official Minutes" → "Unofficial Minutes" and strips "Minutes Prepared by..." lines. Not currently wired into the main pipeline (the Facebook formatter in `pkg/facebook` has its own renderer). Available for future use.
