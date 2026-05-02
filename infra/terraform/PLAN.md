# Terraform IaC Plan — Milton Meeting Summarizer

## Overview

This directory contains Terraform configuration to manage the complete GCP infrastructure
for the Milton Meeting Summarizer project. The goal is to bring the existing live deployment
under Terraform state management (import), then use Terraform as the authoritative source
of truth for all infrastructure going forward.

---

## File Structure

| File | Purpose |
|---|---|
| `main.tf` | Terraform version constraints, GCS backend, google provider |
| `variables.tf` | Input variables with production defaults |
| `outputs.tf` | Key resource references (function URL, bucket name, etc.) |
| `apis.tf` | `google_project_service` — GCP API enablement |
| `iam.tf` | Service accounts and project-level IAM bindings |
| `storage.tf` | GCS bucket for Hugo content staging |
| `pubsub.tf` | Pub/Sub topic for pipeline triggers |
| `firestore.tf` | Firestore database (MongoDB compat mode) |
| `secrets.tf` | Secret Manager secret containers + per-secret IAM |
| `function.tf` | Cloud Function Gen 2 (`youtube-webhook`) |
| `cloudbuild.tf` | Cloud Build triggers (hugo deploy + CI push) |
| `scheduler.tf` | Cloud Scheduler job for PubSubHubbub renewal |
| `imports.tf` | Terraform 1.5+ declarative import blocks |

---

## Resources Under Management

### GCP APIs (`apis.tf`)
10 `google_project_service` resources:
- `artifactregistry.googleapis.com`
- `cloudbuild.googleapis.com`
- `cloudfunctions.googleapis.com`
- `cloudscheduler.googleapis.com`
- `firestore.googleapis.com`
- `pubsub.googleapis.com`
- `run.googleapis.com`
- `secretmanager.googleapis.com`
- `storage.googleapis.com`
- `youtube.googleapis.com`

All use `disable_on_destroy = false` to prevent accidental API disablement.

### Service Account + IAM (`iam.tf`)
- `google_service_account.transcript_summarizer` — `transcript-summarizer@miltonmeetingsummarizer.iam.gserviceaccount.com`
- Project-level IAM members for `transcript-summarizer`:
  - `roles/storage.objectCreator` (unconditional)
  - `roles/storage.admin` (unconditional) — see drift notes
  - `roles/pubsub.publisher` (unconditional)
  - `roles/logging.logWriter` (unconditional)
  - `roles/firebase.admin` (unconditional)
  - `roles/datastore.user` (conditional: meetingtranscripts database only)
- Project-level IAM members for Cloud Build default SA (`108319055995@cloudbuild.gserviceaccount.com`):
  - `roles/firebasehosting.admin`
  - `roles/secretmanager.secretAccessor` (project-level) — see drift notes

### Storage (`storage.tf`)
- `google_storage_bucket.hugo_content` — `miltonmeetingsummarizer-hugo-content`
  - Location: `US-CENTRAL1`, Standard class, uniform bucket-level access, 7-day soft delete

### Pub/Sub (`pubsub.tf`)
- `google_pubsub_topic.pipeline_trigger` — `youtube-pipeline-trigger`

### Firestore (`firestore.tf`)
- `google_firestore_database.meetingtranscripts`
  - Type: `FIRESTORE_NATIVE` (with MongoDB wire protocol compatibility enabled)
  - Location: `nam5` (multi-region US; set at creation, cannot be changed)
  - Edition: Enterprise (free tier)
  - Concurrency: Pessimistic

### Secrets (`secrets.tf`)
9 `google_secret_manager_secret` resources (containers only; values are never stored in state):
`facebook-page-id`, `facebook-page-token`, `firebase-ci-token`, `firebase-sa-key`,
`mongodb-uri`, `openai-api-key`, `supadata-api-key`, `transcriptapi-api-key`, `youtube-api-key`

Per-secret IAM (`roles/secretmanager.secretAccessor`) for `transcript-summarizer` on all 9 secrets.

### Cloud Function Gen 2 (`function.tf`)
- `google_cloudfunctions2_function.youtube_webhook`
  - Runtime: `go124`, entry point: `YouTubeWebhook`
  - Memory: 512 MiB, CPU: 0.3333, timeout: 540s, max instances: 60
  - Service account: `transcript-summarizer`
  - 7 secret env vars injected at deploy time
  - **`ignore_changes` on `build_config[0].source`** — source updates managed by `make deploy`

### Cloud Build Triggers (`cloudbuild.tf`)
- `google_cloudbuild_trigger.hugo_build_and_deploy` (Pub/Sub triggered)
  - Trigger ID: `edc8c74f-8d88-49f1-bbf3-2133cf39e1f5`
  - Topic: `youtube-pipeline-trigger`
  - Source: GitHub `kfiles/TranscriptSummarizer` branch `main`
  - Build config: `filename = "cloudbuild.yaml"` (see drift note below)
- `google_cloudbuild_trigger.transcript_summarizer_push` (GitHub push)
  - Trigger ID: `fe85c125-b301-4662-afdb-5b0bd54e06ec`
  - Push to `^main$`, autodetect build config

### Cloud Scheduler (`scheduler.tf`)
- `google_cloud_scheduler_job.renew_youtube_subscription`
  - Schedule: `0 9 */9 * *` UTC — renews YouTube PubSubHubbub every 9 days
  - Target: `https://pubsubhubbub.appspot.com/subscribe`
  - Body: dynamically computed using `google_cloudfunctions2_function.youtube_webhook.service_config[0].uri`

---

## Resources NOT Under Terraform Management

| Resource | Reason |
|---|---|
| Firebase Hosting site | No stable Terraform resource; managed by `firebase deploy` in Cloud Build |
| Firebase project linkage | Pre-existing; no Terraform support for linking existing Firebase projects |
| GitHub App connection to Cloud Build | Requires OAuth flow; cannot be automated via Terraform |
| YouTube PubSubHubbub subscription | External webhook registration; Scheduler handles renewal |
| Secret values | Never stored in TF state; rotated via `gcloud secrets versions add` |
| `gcf-v2-sources-*` GCS bucket | Managed by Cloud Functions infrastructure |
| Auto-managed Pub/Sub subscriptions | `gcb-hugo-build-and-deploy` subscription created by Cloud Build |
| System service agents | Auto-managed by GCP (`gcf-admin-robot`, `gcp-sa-cloudbuild`, etc.) |
| Firebase Admin SDK SA (`firebase-adminsdk-fbsvc`) | Auto-created by Firebase |
| Compute default SA | Auto-created by GCP; used as function build SA |

---

## Drift: Live State vs. setup.sh

These discrepancies were observed by comparing `setup.sh` against live `gcloud` output.
The Terraform config models the **live state** (ground truth), not `setup.sh`.

| # | Description | Live State | setup.sh | Action |
|---|---|---|---|---|
| 1 | `transcript-summarizer` has `roles/storage.admin` in addition to `roles/storage.objectCreator` | Both present | Only `objectCreator` | Modeled in TF; consider removing `storage.admin` post-import |
| 2 | Cloud Build SA has `roles/secretmanager.secretAccessor` at **project level** | Project-level binding present | Per-secret bindings only | Modeled as project-level in TF; per-secret bindings in setup.sh are superseded |
| 3 | `datastore.user` for `transcript-summarizer` is **unconditional** in live state | Unconditional binding | Conditional binding with meetingtranscripts scope | Modeled unconditional. The conditional `datastore.user` binding in the policy belongs to a Firebase-managed `principal://firestore.googleapis.com/...` member, not this SA |
| 4 | `firebase-sa-key` secret exists in live state | Present | Not in setup.sh | Added to Terraform |
| 5 | `hugo-build-and-deploy` trigger has inline build steps | Inline steps (gcloud inlined cloudbuild.yaml at creation) | `--build-config="cloudbuild.yaml"` | TF uses `filename = "cloudbuild.yaml"`; first apply will update trigger to file-based (desired) |
| 6 | `FACEBOOK_ENABLED` env var on function | Present (`true`) | Not in setup.sh (in Makefile) | Added to TF |
| 7 | `LOG_EXECUTION_ID=true` env var on function | Auto-injected by platform | Not set by us | Not in TF; `ignore_changes` prevents diff |
| 8 | `datastore.user` unconditional binding for `transcript-summarizer` also present | Present | Only conditional binding | Not modeled (may be from Firebase init; do not remove without investigation) |
| 9 | `mongodb-uri` per-secret binding for Cloud Build SA | Present | Granted in setup.sh | Covered by project-level binding in TF; per-secret binding is redundant but harmless |

---

## Bootstrap Procedure

### Step 0: Create Terraform State Bucket

The GCS backend bucket must exist before `terraform init`:

```bash
gcloud storage buckets create gs://miltonmeetingsummarizer-tfstate \
  --location=us-central1 \
  --uniform-bucket-level-access \
  --project=miltonmeetingsummarizer
```

### Step 1: Initialize Terraform

```bash
cd infra/terraform
terraform init
```

### Step 2: Plan the Import

Review what will be imported and what diffs Terraform sees:

```bash
terraform plan
```

Expected outcome: all resources imported with minimal diff. Known expected diffs:
- `hugo-build-and-deploy` trigger: inline build → `filename = "cloudbuild.yaml"` (desired)
- `cloud_scheduler_job` body: current URL hardcoded → dynamically computed (same value, no-op)
- Some IAM binding condition title formatting may differ

### Step 3: Apply

```bash
terraform apply
```

This imports all resources into state and applies any diffs. The only resource that will
be modified (not just imported) is the `hugo-build-and-deploy` trigger (build config format).

### Step 4: Verify

```bash
terraform show          # inspect full state
terraform plan          # should show "No changes" after clean apply
```

---

## Known Import Challenges

### Conditional IAM Binding (datastore.user)

The condition title was stored with embedded quotes in GCP (`"meetingtranscripts-writer-role"`).
The import ID in `imports.tf` uses:
```
"miltonmeetingsummarizer roles/datastore.user serviceAccount:transcript-summarizer@... \"meetingtranscripts-writer-role\""
```

If this import fails, try the import without embedded quotes:
```bash
terraform import google_project_iam_member.ts_datastore_user \
  'miltonmeetingsummarizer roles/datastore.user serviceAccount:transcript-summarizer@miltonmeetingsummarizer.iam.gserviceaccount.com meetingtranscripts-writer-role'
```

### Cloud Build Trigger Build Config

The `hugo-build-and-deploy` trigger was created with `--build-config=cloudbuild.yaml` via
gcloud, which inlines the YAML content into the trigger resource. The Terraform config uses
`filename = "cloudbuild.yaml"` which is the preferred approach (trigger reads from repo on
each build). The first apply will update the trigger from inline to file-based. This is safe
and desirable.

### Function Source (ignore_changes)

The `build_config[0].source` block is ignored by Terraform. The Cloud Function source is
managed by `make deploy` (gcloud CLI), not Terraform. If the source block causes a diff
during plan despite `ignore_changes`, add `build_config` to the ignore list temporarily
during import, then narrow it back to just `source` afterward.

### Unconditional `datastore.user` Binding

The live IAM policy has an unconditional `roles/datastore.user` binding for
`transcript-summarizer` that is not in `setup.sh`. This may have been auto-created by
Firebase initialization. It is **not** modeled in Terraform. If Terraform tries to remove it,
stop and investigate before proceeding — it may be required for Firebase Admin SDK operations.

---

## Post-Import Cleanup Recommendations

| Item | Recommendation |
|---|---|
| `roles/storage.admin` on transcript-summarizer | Downgrade to `roles/storage.objectCreator` only — the function only needs to write objects |
| Conditional title with embedded quotes | Recreate the `datastore.user` binding with a clean title (no embedded quotes) |
| `mongodb-uri` per-secret Cloud Build binding | Remove redundant per-secret binding now superseded by project-level binding |
| Weird conditional `secretmanager.secretAccessor` binding | Remove — condition references a Firestore database, which never matches a Secret Manager resource; binding is effectively dead |

---

## Ongoing Workflow

| Task | How |
|---|---|
| Deploy new function code | `make deploy` (continues to use gcloud CLI) |
| Change function env vars / scaling | Edit `function.tf`, run `terraform apply` |
| Add a new secret | Add to `all_secrets` + `function_secrets` in `secrets.tf`, set value via `gcloud secrets versions add`, then `terraform apply` |
| Rotate a secret value | `echo -n "new" \| gcloud secrets versions add SECRET_NAME --data-file=- --project=miltonmeetingsummarizer` |
| Add/remove IAM binding | Edit `iam.tf`, run `terraform apply` |
| Update Cloud Scheduler | Edit `scheduler.tf`, run `terraform apply` |
| Full infra teardown | Do NOT use `terraform destroy` without removing `disable_on_destroy = false` from APIs first |

---

## Fields Determined from Live GCP State

The following values were interrogated from live GCP state (not derivable from setup.sh or docs):

| Field | Value | Source |
|---|---|---|
| Project number | `108319055995` | `gcloud projects describe` |
| Cloud Build SA email | `108319055995@cloudbuild.gserviceaccount.com` | Derived from project number |
| Firestore location | `nam5` (not `us-central1`) | `gcloud firestore databases list` |
| Firestore concurrency mode | `PESSIMISTIC` | `gcloud firestore databases list` |
| Function service URI | `https://youtube-webhook-eqk57gsoya-uc.a.run.app` | `gcloud run services describe` |
| Function CF URL | `https://us-central1-miltonmeetingsummarizer.cloudfunctions.net/youtube-webhook` | `gcloud functions describe` |
| Function revision | `youtube-webhook-00017-zin` | `gcloud functions describe` |
| Function available CPU | `0.3333` | `gcloud functions describe` |
| Function max instance request concurrency | `1` | `gcloud functions describe` |
| Artifact Registry repo | `projects/miltonmeetingsummarizer/locations/us-central1/repositories/gcf-artifacts` | `gcloud artifacts repositories describe` |
| Build SA | `108319055995-compute@developer.gserviceaccount.com` | `gcloud functions describe` |
| Source bucket | `gcf-v2-sources-108319055995-us-central1` | `gcloud functions describe` |
| hugo-build trigger ID | `edc8c74f-8d88-49f1-bbf3-2133cf39e1f5` | `gcloud builds triggers list` |
| push trigger ID | `fe85c125-b301-4662-afdb-5b0bd54e06ec` | `gcloud builds triggers list` |
| Scheduler body (decoded) | `hub.callback=https://youtube-webhook-eqk57gsoya-uc.a.run.app&hub.topic=...&hub.mode=subscribe&hub.verify=async` | `gcloud scheduler jobs describe` |
| GCS bucket soft delete retention | `604800s` (7 days) | `gcloud storage buckets describe` |
| `firebase-sa-key` secret | Exists (not in setup.sh) | `gcloud secrets list` |
| `FACEBOOK_ENABLED` env var | `true` (in Makefile, not setup.sh) | `gcloud functions describe` |
| Condition title has embedded quotes | Title stored as `"meetingtranscripts-writer-role"` | `gcloud projects get-iam-policy` |
| storage.admin granted to transcript-summarizer | Both `storage.admin` + `storage.objectCreator` present | `gcloud projects get-iam-policy` |
| Cloud Build SA has project-level secret accessor | `roles/secretmanager.secretAccessor` at project level | `gcloud projects get-iam-policy` |
