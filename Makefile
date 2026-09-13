GO ?= go

.PHONY: test test-unit

# Complete suite, including Ebitengine integration tests. The wrapper supplies
# a virtual X11 display automatically on headless Linux systems.
test:
	GO="$(GO)" ./scripts/test.sh ./...

# Fast display-independent packages for normal edit/test cycles.
test-unit:
	GO="$(GO)" ./scripts/test-unit.sh
