GO_FILES=$(shell find . \( -path "./.go_pkg_cache" -o -path "./data" \) -prune -o -name "*.go" -print)

.PHONY: fmt lint dev shell test build ctags db schema run help

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

#? ctags: Generates tags for golang files
ctags:
	ctags -R ${GO_FILES}

#? db: Opens postgres cli
db:
	PGPASSWORD=dev psql -U postgres -p 5432 -h db -d postgres

#? db: Creates the database schema
schema:
	PGPASSWORD=dev psql -U postgres -p 5432 -h db -d postgres < ./schema/01_initial_schema.sql

#? run: Runs local web server
run:
	env LOG_LEVEL=$(LOG_LEVEL)  go run ./cmd/server/main.go


#? help: Get more info on make commands.
help: Makefile
	@echo ''
	@echo 'Usage:'
	@echo '  make [target]'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^#?//p' $< | column -t -s ':' |  sort | sed -e 's/^/ /'
