# C4 Level 1 — System Context

This diagram shows the Milton Meeting Summarizer as a single system and documents every external actor and system that it interacts with. No internal detail is shown at this level.

> **Rendering:** Open in VS Code with the PlantUML extension (`Alt+D`), or run `java -jar plantuml.jar docs/architecture/c1-context.md`.

```plantuml
@startuml C4_Context_Milton_Summarizer
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Context.puml

LAYOUT_WITH_LEGEND()
SHOW_PERSON_OUTLINE()

title System Context — Milton Meeting Summarizer

' ── Actors ──────────────────────────────────────────────────────────────────
Person(resident, "Milton Resident", "Views unofficial meeting minutes\nand AI-generated summaries on the\npublic website")

Person(admin, "System Administrator", "Operates the pipeline: manages\nplaylists and officials in Firestore,\nrotates secrets, deploys Cloud Function")

' ── System under description ─────────────────────────────────────────────────
System(summarizer, "Milton Meeting Summarizer", "Monitors the Milton, MA YouTube channel\nfor new town meeting recordings.\nTranscribes, summarizes, and publishes\nmeeting minutes automatically.")

' ── External systems ─────────────────────────────────────────────────────────
System_Ext(youtube_channel, "YouTube\n(Milton Channel)", "Channel ID: UCGnv43oWpciURP-bTDc3GnA\nHosts town meeting recordings\nfor Select Board, Planning Board,\nSchool Committee, and others")

System_Ext(pshub, "YouTube PubSubHubbub Hub", "pubsubhubbub.appspot.com\nWebSub-compliant hub that delivers\nHTTP push notifications to subscribers\nwhen new videos are published")

System_Ext(ytdataapi, "YouTube Data API v3", "googleapis.com/youtube/v3\nProvides playlist metadata and\nvideo details (title, publish date,\nposition). No OAuth required;\nuses a public API key.")

System_Ext(transcriptapi, "TranscriptAPI.com", "transcriptapi.com/api/v2\nPrimary third-party transcript\nprovider. Returns plain-text\ntranscript for a YouTube video ID.\nAuthentication: Bearer token.")

System_Ext(supadata, "Supadata", "api.supadata.ai/v1\nFallback transcript provider.\nSupports async job polling for\nlong videos. Auth: x-api-key header.")

System_Ext(openai, "OpenAI API", "api.openai.com — Model: gpt-4.1-mini\nReceives raw transcript text and\nan optional list of official names.\nReturns structured Markdown\nformatted as unofficial meeting minutes.")

System_Ext(facebook, "Facebook Graph API v22.0", "graph.facebook.com/v22.0/<pageId>/feed\nReceives a plain-text post containing\nthe meeting summary and a link to\nthe full transcript on the website.\nAuth: Page access token.")

System_Ext(miltonma, "miltonma.gov", "miltonma.gov/890/Town-Wide\nHTML page listing all town committees\nand their appointed members.\nScraped by the admin tool to populate\nthe officials Firestore collection.")

System_Ext(github, "GitHub", "Source repository for this project.\nCode push to main branch triggers\nthe transcript-summarizer-push\nCloud Build trigger (site CI).")

' ── Relationships ─────────────────────────────────────────────────────────────
Rel(youtube_channel, pshub, "Publishes new-video events\n(WebSub protocol)")
Rel(pshub, summarizer, "HTTP POST Atom/XML notification\n(unauthenticated, public endpoint)\nwhen a new video is published")
Rel(summarizer, pshub, "HTTP POST subscription request\nevery 9 days via Cloud Scheduler\n(hub.mode=subscribe)")
Rel(summarizer, ytdataapi, "Fetches playlist items and\nvideo metadata by playlist ID\n(API key, no OAuth)")
Rel(summarizer, transcriptapi, "GET transcript text for video ID\n(primary provider, Bearer auth)")
Rel(summarizer, supadata, "GET transcript text for video ID\n(fallback provider, API key auth)")
Rel(summarizer, openai, "POST transcript text + official names;\nreceives Markdown summary\n(model: gpt-4.1-mini)")
Rel(summarizer, facebook, "POST plain-text meeting summary\nwith link to transcript page\n(Page access token auth)")
Rel(admin, miltonma, "Runs cmd/officials admin tool\nto scrape committee rosters")
Rel(summarizer, miltonma, "HTTP GET Town-Wide page HTML\n(via admin tool, not Cloud Function)")
Rel(github, summarizer, "git push to main triggers\nCloud Build site rebuild")
Rel(resident, summarizer, "HTTPS — reads meeting summaries\nat miltonmeetingsummarizer.web.app")
Rel(admin, summarizer, "Deploys Cloud Function, manages\nFirestore playlists and officials,\nrotates Secret Manager credentials")

@enduml
```

---

## External System Reference

### YouTube Channel

- **Channel ID:** `UCGnv43oWpciURP-bTDc3GnA`
- **Relationship:** Source of all video content. The system does not poll the channel; it receives push notifications from the PubSubHubbub hub.
- **Playlists tracked:** Select Board 2026, Planning Board 2026, Conservation Commission 2026, School Committee 2026, Warrant Committee 2026. These are managed in the `playlists` Firestore collection and seeded by `cmd/syncplaylists`.

### YouTube PubSubHubbub Hub

- **URL:** `https://pubsubhubbub.appspot.com/subscribe`
- **Protocol:** WebSub / PubSubHubbub 0.4
- **Subscription lifetime:** ~10 days. Cloud Scheduler renews every 9 days.
- **Notification format:** Atom XML feed containing `<yt:videoId>` and `<yt:channelId>` elements.
- **Verification:** Hub sends a `GET` with `hub.challenge` query param; the function echoes it back verbatim to confirm the subscription.
- **Note:** The hub delivers to an unauthenticated HTTP endpoint. There is no HMAC signature verification implemented; the function trusts any POST to the webhook URL.

### YouTube Data API v3

- **Auth:** Public API key (`YOUTUBE_API_KEY` secret)
- **Used for:** `playlistItems.list` (to scan playlist contents) and `videos.list` (to fetch single video metadata by ID). No user OAuth is needed; all data is public.
- **SDK:** `google.golang.org/api/youtube/v3`

### TranscriptAPI.com (Primary Transcript Provider)

- **Base URL:** `https://transcriptapi.com/api/v2`
- **Auth:** `Authorization: Bearer <TRANSCRIPTAPI_API_KEY>`
- **Behavior:** Synchronous GET. Returns 200 with transcript or 404 if no transcript is available for the video. Retries up to 3 times on 408/429/503.
- **Selected by:** `TRANSCRIPT_PROVIDER=transcriptapi` (set in Cloud Function deployment)

### Supadata (Fallback Transcript Provider)

- **Base URL:** `https://api.supadata.ai/v1`
- **Auth:** `x-api-key: <SUPADATA_API_KEY>`
- **Behavior:** May return 200 (synchronous) or 202 with a `jobId` (async). For async responses, the transcriber polls the job endpoint at 1-second intervals until `status=completed`.
- **Selected by:** `TRANSCRIPT_PROVIDER=supadata` or empty

### OpenAI API

- **Model:** `gpt-4.1-mini`
- **Auth:** `CHATGPT_API_KEY` (via Secret Manager)
- **Input:** Raw transcript text + a list of official names for spelling correction
- **Output:** Structured Markdown formatted as unofficial meeting minutes, organized by discussion topic with bullet points
- **System prompt:** "You are a professional town reporter reporting unofficial minutes of town meetings. Use formal language, titles, and observe all official motions and votes..."

### Facebook Graph API v22.0

- **Endpoint:** `POST https://graph.facebook.com/v22.0/{FACEBOOK_PAGE_ID}/feed`
- **Auth:** `access_token=<FACEBOOK_PAGE_TOKEN>` (form field)
- **Post format:** Meeting title, plain-text summary (Markdown converted via custom renderer), and a link to the full transcript on the website.
- **URL format:** `https://miltonmeetingsummarizer.web.app/minutes/{YYYY}/{Month}/{videoId}/`
- **Disabling:** Set `FACEBOOK_ENABLED=false` to skip Facebook posting without removing credentials.

### miltonma.gov

- **URL:** `https://www.miltonma.gov/890/Town-Wide`
- **Used by:** `cmd/officials` admin tool (not the Cloud Function directly)
- **Purpose:** Provides the roster of appointed committee members. These names are stored in Firestore and fetched at summarization time so OpenAI can spell them correctly.
- **Parsing strategy:** Deterministic DOM parser (`ParseTownWideDOM`) targeting CivicPlus widget CSS classes. LLM fallback (`LLMExtractor.ParseTownWideLLM`) available if the page structure changes.

### GitHub

- **Triggers:** `transcript-summarizer-push` Cloud Build trigger (on push to `main` branch)
- **Purpose:** Code CI — not part of the meeting pipeline. The build trigger for the pipeline (`hugo-build-and-deploy`) is Pub/Sub-triggered, not GitHub-triggered.
