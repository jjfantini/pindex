# pindex — dev convenience targets. The release pipeline is described in
# docs/RELEASING.md; release artifacts are produced in CI, not here.
.PHONY: setup-hooks test test-static lint snapshot help

help:
	@echo "make setup-hooks  - enable the Conventional-Commit pre-push hook (.githooks)"
	@echo "make test         - go test -race ./..."
	@echo "make test-static  - CGO_ENABLED=0 go test ./...  (pure-Go path)"
	@echo "make lint         - go vet + golangci-lint"
	@echo "make snapshot     - local goreleaser snapshot of the pure-Go (lite) build"

# Point git at .githooks and install commitsar (best-effort; the hook has a
# regex fallback if commitsar is unavailable).
setup-hooks:
	@git config core.hooksPath .githooks
	@command -v commitsar >/dev/null 2>&1 || \
		go install github.com/aevea/commitsar@latest 2>/dev/null || \
		echo "note: could not install commitsar; the pre-push hook will use its regex fallback."
	@echo "git hooks enabled (core.hooksPath=.githooks) — Conventional-Commit lint on push."

test:
	go test -race ./...

test-static:
	CGO_ENABLED=0 go test ./...

lint:
	go vet ./...
	golangci-lint run

# Build + archive the pure-Go (lite) artifacts locally; publishes nothing.
snapshot:
	goreleaser release --snapshot --clean
