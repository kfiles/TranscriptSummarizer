PROJECT_ID       ?= miltonmeetingsummarizer
REGION           ?= us-central1
FACEBOOK_ENABLED ?= true
SA_EMAIL         := transcript-summarizer@$(PROJECT_ID).iam.gserviceaccount.com
CONTENT_BUCKET   := $(PROJECT_ID)-hugo-content
PUBSUB_TOPIC     := youtube-pipeline-trigger

.PHONY: test test-verbose test-coverage test-integration build-officials officials deploy deploy-no-facebook

test:
	go test ./pkg/...

test-verbose:
	go test -v ./pkg/...

test-coverage:
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

# Requires MONGODB_URI to be set and a reachable MongoDB instance.
test-integration:
	go test -tags integration ./pkg/...

build-officials:
	go build -o bin/officials ./cmd/officials/

officials: build-officials
	./bin/officials

deploy:
	gcloud functions deploy youtube-webhook \
		--gen2 \
		--runtime=go124 \
		--region=$(REGION) \
		--source=. \
		--entry-point=YouTubeWebhook \
		--trigger-http \
		--allow-unauthenticated \
		--service-account=$(SA_EMAIL) \
		--timeout=540s \
		--memory=512Mi \
		--set-env-vars="HUGO_CONTENT_DIR=/tmp/hugo-content/minutes,GCS_BUCKET=$(CONTENT_BUCKET),PUBSUB_PROJECT=$(PROJECT_ID),PUBSUB_TOPIC=$(PUBSUB_TOPIC),FACEBOOK_ENABLED=$(FACEBOOK_ENABLED)" \
		--set-secrets="MONGODB_URI=mongodb-uri:latest,CHATGPT_API_KEY=openai-api-key:latest,FACEBOOK_PAGE_ID=facebook-page-id:latest,FACEBOOK_PAGE_TOKEN=facebook-page-token:latest,YOUTUBE_API_KEY=youtube-api-key:latest,SUPADATA_API_KEY=supadata-api-key:latest" \
		--project=$(PROJECT_ID)

deploy-no-facebook: FACEBOOK_ENABLED=false
deploy-no-facebook: deploy
