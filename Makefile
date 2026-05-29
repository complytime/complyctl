GO_BUILD_PACKAGES := ./cmd/...
GO_BUILD_BINDIR :=./bin
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG ?= $(shell git tag | sort -V | tail -1 2>/dev/null || echo "v0.0.0")
GIT_TREE_STATE ?= $(shell test -n "`git status --porcelain 2>/dev/null`" && echo "dirty" || echo "clean")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

GO_LD_EXTRAFLAGS := -X github.com/complytime/complyctl/internal/version.version="$(GIT_TAG)" \
                    -X github.com/complytime/complyctl/internal/version.gitTreeState=$(GIT_TREE_STATE) \
                    -X github.com/complytime/complyctl/internal/version.commit="$(GIT_COMMIT)" \
                    -X github.com/complytime/complyctl/internal/version.buildDate="$(BUILD_DATE)"

MAN_COMPLYCTL = docs/man/complyctl.md
MAN_COMPLYCTL_OUTPUT = docs/man/complyctl.1

##@ Proto

proto: ## generate protobuf code (requires buf)
	@command -v buf >/dev/null 2>&1 && buf generate || \
		echo "Install buf: https://buf.build/docs/installation"

lint-proto: ## lint protobuf files with buf
	@command -v buf >/dev/null 2>&1 && buf lint || \
		echo "Install buf: https://buf.build/docs/installation"
.PHONY: lint-proto

##@ Mock Servers (for testing)

mock-registry: ## start mock OCI registry on port 8765
	go run ./cmd/mock-oci-registry

mock-registry-background: ## start mock OCI registry in background
	@go run ./cmd/mock-oci-registry & \
	echo "Mock registry PID: $$!"; \
	echo "Stop with: kill $$!"

test-e2e: build build-test-provider ## run full E2E tests (in-process mock registry + test provider)
	go test -tags=e2e -mod=vendor ./tests/e2e/... -v -count=1 -timeout 120s

test-behavioral: build build-test-provider build-behavioral-report ## run behavioral assessment and generate EvaluationLog + SARIF
	$(GO_BUILD_BINDIR)/behavioral-report \
		-binary $(GO_BUILD_BINDIR)/complyctl \
		-test-provider $(GO_BUILD_BINDIR)/complytime-provider-test \
		-catalog governance/controls/complytime-controls.yaml \
		-artifact-uri governance/controls/complytime-controls.yaml \
		-out governance/reports
.PHONY: test-behavioral

test-integration: build build-test-provider ## run integration test (mock registry + test provider, shell-based)
	./tests/integration_test.sh
.PHONY: test-integration

test-cross-repo: build ## run cross-repo integration test (requires PROVIDERS_BIN_DIR and GITHUB_TOKEN)
ifndef PROVIDERS_BIN_DIR
	$(error PROVIDERS_BIN_DIR is not set. Set it to the directory containing complyctl-provider-ampel)
endif
	timeout 120 ./tests/cross-repo/cross_repo_integration_test.sh
.PHONY: test-cross-repo

test-devcontainer: ## verify devcontainer Containerfile builds
	podman build -t complyctl-devcontainer-test .devcontainer/
	@echo "Containerfile builds successfully."
.PHONY: test-devcontainer

##@ Compilation

all: clean vendor test-unit build ## compile from scratch
.PHONY: all

build: prep-build-dir ## compile
	go build -mod=vendor -o $(GO_BUILD_BINDIR)/ -ldflags="$(GO_LD_EXTRAFLAGS)" $(GO_BUILD_PACKAGES)
.PHONY: build

build-test-provider: prep-build-dir ## build test provider for E2E tests
	go build -mod=vendor -o $(GO_BUILD_BINDIR)/complyctl-provider-test ./cmd/test-provider
.PHONY: build-test-provider

build-behavioral-report: prep-build-dir ## build behavioral report tool (go test -json -> EvaluationLog + SARIF)
	go build -mod=vendor -o $(GO_BUILD_BINDIR)/behavioral-report ./cmd/behavioral-report
.PHONY: build-behavioral-report

##@ Packaging

man: ## generate man pages
	mkdir -p $(dir $(MAN_COMPLYCTL_OUTPUT))
	pandoc -s -t man $(MAN_COMPLYCTL) -o $(MAN_COMPLYCTL_OUTPUT)

##@ Environment

dev-setup: dev-setup-commit-hooks ## prepare workspace for contributing
.PHONY: dev-setup

dev-setup-commit-hooks: ## configure pre-commit
	pre-commit install --hook-type pre-commit --hook-type pre-push
.PHONY: dev-setup-commit-hooks

prep-build-dir: ## create build output directory
	mkdir -p ${GO_BUILD_BINDIR}
.PHONY: prep-build-dir

vendor: ## go mod sync
	go mod tidy
	go mod verify
	go mod vendor
.PHONY: vendor

clean:
	@rm -rf ./$(GO_BUILD_BINDIR)/*
	rm -f $(MAN_COMPLYCTL_OUTPUT)
.PHONY: clean

##@ Testing

test-unit:
	go test -race -v -coverprofile=coverage.out ./...
.PHONY: test-unit

sanity: vendor format vet ## ensure code is ready for commit
	git diff --exit-code
.PHONY: sanity

format:
	go fmt ./...
.PHONY: format

vet:
	go vet ./...
.PHONY: vet

lint: ## run linters (golangci-lint + goimports check)
	golangci-lint run ./...
	@command -v goimports >/dev/null 2>&1 && goimports -l ./internal/ ./cmd/ || true
.PHONY: lint

##@ CRAP Load Monitoring

GAZE_VERSION ?= latest
GAZE_BASELINE := .gaze/baseline.json
GAZE_COVERPROFILE := coverage.out
GAZE_NEW_FUNC_THRESHOLD ?= 30

ensure-gaze: ## install gaze if not present
	@command -v gaze >/dev/null 2>&1 || \
		(echo "Installing gaze..." && go install github.com/unbound-force/gaze/cmd/gaze@$(GAZE_VERSION))
.PHONY: ensure-gaze

crapload: ensure-gaze test-unit ## run CRAP and GazeCRAP analysis (human-readable)
	gaze crap --format=text --coverprofile=$(GAZE_COVERPROFILE) ./...
.PHONY: crapload

crapload-baseline: ensure-gaze test-unit ## generate baseline thresholds in .gaze/baseline.json
	@mkdir -p .gaze
	@REPO_ROOT=$$(pwd); \
	gaze crap --format=json --coverprofile=$(GAZE_COVERPROFILE) ./... | \
		jq --arg root "$$REPO_ROOT/" '(.scores[],.summary.worst_crap[]?,.summary.worst_gaze_crap[]?) |= (.file |= ltrimstr($$root))' > $(GAZE_BASELINE)
	@echo "Baseline written to $(GAZE_BASELINE)"
.PHONY: crapload-baseline

crapload-check: ensure-gaze test-unit ## check for CRAP regressions against baseline
	@if [ ! -f $(GAZE_BASELINE) ]; then \
		echo "ERROR: Baseline file $(GAZE_BASELINE) not found. Run 'make crapload-baseline' first."; \
		exit 1; \
	fi
	@REPO_ROOT=$$(pwd); \
	gaze crap --format=json --coverprofile=$(GAZE_COVERPROFILE) ./... | \
		jq --arg root "$$REPO_ROOT/" '(.scores[],.summary.worst_crap[]?,.summary.worst_gaze_crap[]?) |= (.file |= ltrimstr($$root))' > /tmp/crapload-current.json
	@echo "Comparing against baseline..."
	@jq -r '.scores[] | "\(.file):\(.function) \(.crap) \(.gaze_crap // 0)"' $(GAZE_BASELINE) | sort > /tmp/crapload-baseline.txt
	@jq -r '.scores[] | "\(.file):\(.function) \(.crap) \(.gaze_crap // 0)"' /tmp/crapload-current.json | sort > /tmp/crapload-current.txt
	@REGRESSIONS=0; \
	while IFS=' ' read -r func crap gaze_crap; do \
		baseline_crap=$$(grep -F "$$func " /tmp/crapload-baseline.txt | head -1 | awk '{print $$2}'); \
		baseline_gaze=$$(grep -F "$$func " /tmp/crapload-baseline.txt | head -1 | awk '{print $$3}'); \
		if [ -z "$$baseline_crap" ]; then \
			if [ "$$(echo "$$crap > $(GAZE_NEW_FUNC_THRESHOLD)" | bc -l)" = "1" ]; then \
				echo "NEW FUNCTION VIOLATION: $$func CRAP=$$crap (threshold=$(GAZE_NEW_FUNC_THRESHOLD))"; \
				REGRESSIONS=$$((REGRESSIONS + 1)); \
			fi; \
		else \
			if [ "$$(echo "$$crap > $$baseline_crap" | bc -l)" = "1" ]; then \
				echo "REGRESSION: $$func CRAP $$baseline_crap -> $$crap"; \
				REGRESSIONS=$$((REGRESSIONS + 1)); \
			fi; \
			if [ "$$(echo "$$gaze_crap > $$baseline_gaze" | bc -l)" = "1" ]; then \
				echo "REGRESSION: $$func GazeCRAP $$baseline_gaze -> $$gaze_crap"; \
				REGRESSIONS=$$((REGRESSIONS + 1)); \
			fi; \
		fi; \
	done < /tmp/crapload-current.txt; \
	if [ $$REGRESSIONS -gt 0 ]; then \
		echo "FAIL: $$REGRESSIONS regression(s) detected"; \
		exit 1; \
	else \
		echo "PASS: No regressions detected"; \
	fi
.PHONY: crapload-check

##@ Help

GREEN := \033[0;32m
TEAL := \033[0;36m
CLEAR := \033[0m

help: ## Show this help.
	@printf "Usage: make $(GREEN)<target>$(CLEAR)\n"
	@awk -v "green=${GREEN}" -v "teal=${TEAL}" -v "clear=${CLEAR}" -F ":.*## *" \
			'/^[a-zA-Z0-9_-]+:/{sub(/:.*/,"",$$1);printf "  %s%-12s%s %s\n", green, $$1, clear, $$2} /^##@/{printf "%s%s%s\n", teal, substr($$1,5), clear}' $(MAKEFILE_LIST)
.PHONY: help
