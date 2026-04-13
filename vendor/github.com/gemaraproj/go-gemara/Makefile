# Makefile for go-gemara
# Purpose: provide convenient local targets for building, testing and basic CI parity.

REPO_ROOT := $(shell git rev-parse --show-toplevel)
PKGS := ./...
BINDIR := ./bin
# Binaries to build (paths to cmd packages)
BINS := ./cmd/oscalexport ./cmd/typestagger
GOFLAGS :=
COVERFILE := coverage.out
TESTCOVERAGE_THRESHOLD := 71
GOLANGCI_LINT := golangci-lint
SPECVERSION := v1.0.0

.PHONY: all tidy fmtcheck fmt vet lint test testcov race coverage-check build install generate ci-local clean help

# Default target
all: tidy fmtcheck vet lint testcov build
	@echo "All done."

# Run `go mod tidy` at repo root (required because module files live at repo root)
tidy:
	@echo " > Tidying module at repo root ($(REPO_ROOT))"
	@cd $(REPO_ROOT) && go mod tidy

# Formatting check (fail if any files are unformatted)
fmtcheck:
	@echo " > Checking gofmt"
	@sh -c "test -z \"$$(gofmt -l .)\" || (echo 'gofmt -l found non-formatted files:' && gofmt -l . && exit 1)"

# Apply gofmt (destructive)
fmt:
	@echo " > Formatting files with gofmt"
	@gofmt -w .

vet:
	@echo " > Running go vet (monorepo-aware; will skip if module not present)"
	@sh -c '\
	if [ -d "$(REPO_ROOT)/go-gemara" ]; then \
	  cd $(REPO_ROOT)/go-gemara && go list ./... >/dev/null 2>&1 && go vet ./... || echo "Skipping vet: no module in go-gemara"; \
	elif [ -f "$(REPO_ROOT)/go.mod" ]; then \
	  cd $(REPO_ROOT) && go list ./... >/dev/null 2>&1 && go vet ./... || echo "Skipping vet: module not available"; \
	else \
	  cd $(REPO_ROOT) && go list ./go-gemara/... >/dev/null 2>&1 && go vet ./go-gemara/... || echo "Skipping vet: package not available"; \
	fi'




lint:
	@echo " > Running golangci-lint (monorepo-aware; will skip if module not present)"
	@sh -c '\
	if [ -d "$(REPO_ROOT)/go-gemara" ]; then \
	  cd $(REPO_ROOT)/go-gemara && go list ./... >/dev/null 2>&1 && $(GOLANGCI_LINT) run ./... || echo "Skipping lint: no module in go-gemara"; \
	elif [ -f "$(REPO_ROOT)/go.mod" ]; then \
	  cd $(REPO_ROOT) && go list ./... >/dev/null 2>&1 && $(GOLANGCI_LINT) run ./... || echo "Skipping lint: module not available"; \
	else \
	  cd $(REPO_ROOT) && go list ./go-gemara/... >/dev/null 2>&1 && $(GOLANGCI_LINT) run ./go-gemara/... || echo "Skipping lint: package not available"; \
	fi'




# Run unit tests
test:
	@echo " > Running go test (monorepo-aware; will skip if module not present)"
	@sh -c '\
	if [ -d "$(REPO_ROOT)/go-gemara" ]; then \
	  cd $(REPO_ROOT)/go-gemara && if go list ./... >/dev/null 2>&1; then go test $(GOFLAGS) ./...; else echo "Skipping tests: no module in go-gemara"; fi; \
	elif [ -f "$(REPO_ROOT)/go.mod" ]; then \
	  cd $(REPO_ROOT) && if go list ./... >/dev/null 2>&1; then go test $(GOFLAGS) ./...; else echo "Skipping tests: module not available"; fi; \
	else \
	  cd $(REPO_ROOT) && if go list ./go-gemara/... >/dev/null 2>&1; then go test $(GOFLAGS) ./go-gemara/...; else echo "Skipping tests: package not available"; fi; \
	fi'




# Run tests and write coverage
testcov:
	@echo " > Running tests with coverage (monorepo-aware; will skip if module not present)"
	@sh -c '\
	if [ -d "$(REPO_ROOT)/go-gemara" ]; then \
	  cd $(REPO_ROOT)/go-gemara && if go list ./... >/dev/null 2>&1; then go test $(GOFLAGS) ./... -coverprofile=$(abspath $(COVERFILE)) -covermode=count; else echo "Skipping testcov: no module in go-gemara"; fi; \
	elif [ -f "$(REPO_ROOT)/go.mod" ]; then \
	  cd $(REPO_ROOT) && if go list ./... >/dev/null 2>&1; then go test $(GOFLAGS) ./... -coverprofile=$(abspath $(COVERFILE)) -covermode=count; else echo "Skipping testcov: module not available"; fi; \
	else \
	  cd $(REPO_ROOT) && if go list ./go-gemara/... >/dev/null 2>&1; then go test $(GOFLAGS) ./go-gemara/... -coverprofile=$(abspath $(COVERFILE)) -covermode=count; else echo "Skipping testcov: package not available"; fi; \
	fi'
	@echo " > Coverage summary:"
	@sh -c 'if [ -f "$(abspath $(COVERFILE))" ]; then go tool cover -func=$(abspath $(COVERFILE)) | grep total || true; else echo "No coverage file generated"; fi'



race:
	@echo " > Running tests with race detector (monorepo-aware; will skip if module not present)"
	@sh -c '\
	if [ -d "$(REPO_ROOT)/go-gemara" ]; then \
	  cd $(REPO_ROOT)/go-gemara && if go list ./... >/dev/null 2>&1; then go test -race ./...; else echo "Skipping race: no module in go-gemara"; fi; \
	elif [ -f "$(REPO_ROOT)/go.mod" ]; then \
	  cd $(REPO_ROOT) && if go list ./... >/dev/null 2>&1; then go test -race ./...; else echo "Skipping race: module not available"; fi; \
	else \
	  cd $(REPO_ROOT) && if go list ./go-gemara/... >/dev/null 2>&1; then go test -race ./go-gemara/...; else echo "Skipping race: package not available"; fi; \
	fi'


# Check coverage threshold (requires testcov)
coverage-check:
	@echo " > Checking coverage threshold ($(TESTCOVERAGE_THRESHOLD)%)"
	@sh -c "COVFILE='$(abspath $(COVERFILE))'; \
	if [ ! -f \"$$COVFILE\" ]; then \
	  echo \"$$COVFILE not found; skipping coverage-check (run make testcov first to create coverage file)\"; exit 0; \
	fi; \
	cov=$$(go tool cover -func=$$COVFILE | awk '/total/ {gsub("%","",$$3); print $$3}'); \
	comp=$$(awk -v c=\"$$cov\" -v t=\"$(TESTCOVERAGE_THRESHOLD)\" 'BEGIN { if (c+0 < t+0) print 1; else print 0 }'); \
	if [ \"$$comp\" -eq 1 ]; then \
	  echo \"Coverage $$cov% is below threshold $(TESTCOVERAGE_THRESHOLD)%\"; exit 1; \
	else \
	  echo \"Coverage $$cov% meets threshold $(TESTCOVERAGE_THRESHOLD)%\"; \
	fi"


# Build CLI binaries listed in BINS
build:
	@echo " > Building binaries to $(BINDIR)"
	@mkdir -p $(BINDIR)
	@for b in $(BINS); do \
	  bn=$$(basename $$b); \
	  echo "  - building $$b -> $(BINDIR)/$$bn"; \
	  go build -o $(BINDIR)/$$bn $$b || exit 1; \
	done

# Install package/binaries (simple wrapper)
install:
	@echo " > Installing module/binaries"
	@go install ./...

# Generate files from CUE schemas
# Generates Go types from the Gemara CUE package with stable and experimental variants
generate:
	@echo " > Generating types from Gemara CUE package"
	@cue exp gengotypes --outfile generated_types.go github.com/gemaraproj/gemara@$(SPECVERSION)
	@go run ./cmd/typestagger generated_types.go

genlocal:
	@echo " > Generating types from 'gemara' package"
	@cue exp gengotypes ../gemara:gemara
	@mv ../gemara/cue_types_gemara_gen.go generated_types.go
	@go run ./cmd/typestagger generated_types.go

# Runs the small subset used by CI for a quick local check
ci-local: fmtcheck vet lint testcov coverage-check
	@echo "CI-local checks complete"

clean:
	@echo " > Cleaning build artifacts"
	@rm -rf $(BINDIR) $(COVERFILE) *.coverprofile

oscal-export:
	@echo "  >  Generating OSCAL testdata from Gemara artifacts..."
	@mkdir -p artifacts
	@go run ./cmd/oscalexport catalog ./test-data/good-osps.yml --output ./artifacts/catalog.json
	@go run ./cmd/oscalexport guidance ./test-data/good-aigf.yaml --catalog-output ./artifacts/guidance.json --profile-output ./artifacts/profile.json

help:
	@echo "make targets:"
	@echo "  all            - tidy -> fmtcheck -> vet -> lint -> testcov -> build"
	@echo "  tidy           - run 'go mod tidy' at repo root"
	@echo "  fmtcheck       - fail if formatting issues found"
	@echo "  fmt            - run gofmt -w"
	@echo "  vet            - go vet"
	@echo "  lint           - golangci-lint run ./... (needs tool installed)"
	@echo "  test           - go test"
	@echo "  testcov        - go test -coverprofile=$(COVERFILE)"
	@echo "  coverage-check - ensure coverage >= $(TESTCOVERAGE_THRESHOLD)%"
	@echo "  build          - build binaries listed in BINS -> $(BINDIR)"
	@echo "  generate       - generate Go types"
	@echo "  ci-local       - run quick CI-like checks (fmtcheck vet lint testcov coverage-check)"
	@echo "  clean          - remove build artifacts"
	@echo "  oscal-export    - export to OSCAL from existing Gemara test artifacts"

# TODOs / notes:
# - Consider adding staticcheck or a separate 'staticcheck' target if desired.
# - golangci-lint must be installed locally for 'lint' to succeed; CI uses the golangci-lint GitHub Action.
# - Coverage parsing uses 'awk' and should work on macOS; if you have a different shell environment, adjust accordingly.
# - If you want to add a 'vet' or 'lint' subset, add variables to configure packages.
