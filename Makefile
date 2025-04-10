GO_FILES=$(shell find . \( -path "./.go_pkg_cache" -o -path "./data" \) -prune -o -name "*.go" -print)

.PHONY: dev fmt help lint shell test build release stage tag run run-cron

LOG_LEVEL ?= "INFO"
COIN_GECKO_KEY ?= ""

#? fmt: Ensure consistent code formatting.
fmt:
	gofmt -s -w ${GO_FILES}

#? lint: Run pre-selected linters.
lint:
	golangci-lint run --config .golangci.yml ./cmd/... ./internal/... 

#? dev: Build the dev environment container.
dev:
	./container.sh build

#? shell: Open shell in the dev container.
shell:
	./container.sh run

#? test: Run the tests.
test:
	go test -v ./internal/... ./cmd/...

#? build: Builds the app.
build:
	env CGO_ENABLED=0 go build -o ./build/bin/server -ldflags '-s' ./cmd/server/main.go

#? release: Builds a Docker image with the app and pushes it to the registry.
release:
	$(MAKE) tag TAG=latest

#? stage: Builds a Docker image with the app and pushes it to the registry.
stage:
	$(MAKE) tag TAG=staging

tag:
	@if [ -z "$(TAG)" ]; then \
		$(MAKE) tag TAG=staging; \
	else \
		docker build -t stickers:latest --build-arg NAME=stickersapp -f ./dockerfiles/app/Dockerfile . && \
		docker tag stickers:latest registry.digitalocean.com/dog-registry/stickers-server:$(TAG) && \
		docker push registry.digitalocean.com/dog-registry/stickers-server:$(TAG); \
	fi

#? ctags: Generates tags for golang files
ctags:
	ctags -R ${GO_FILES}

#? db: Opens postgres cli
db:
	PGPASSWORD=dev psql -U postgres -p 5432 -h db -d postgres

#? run: Runs local web server
run:
	env BOT_TOKEN=$(BOT_TOKEN) LOG_LEVEL=$(LOG_LEVEL) WEBHOOK_SECRET=sticky-secret ADMINS=999999 USERS=999999 JWT_SIG_KEY=test GOD=IDDQD go run ./cmd/server/main.go

#? run-cron: Runs the fetchers
run-cron:
	env COIN_GECKO_KEY=$(COIN_GECKO_KEY) LOG_LEVEL=$(LOG_LEVEL) go run ./cmd/crons/main.go

#? help: Get more info on make commands.
help: Makefile
	@echo ''
	@echo 'Usage:'
	@echo '  make [target]'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^#?//p' $< | column -t -s ':' |  sort | sed -e 's/^/ /'
