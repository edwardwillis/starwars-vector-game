GO ?= go

# These targets are intended for development from WSL. The executable is put
# on the Windows filesystem and launched through cmd.exe, so it uses the native
# Windows graphics stack rather than WSLg/XWayland.
WINDOWS_EXE ?= $(shell wslpath -u "$$(cmd.exe /C "echo %USERPROFILE%" 2>/dev/null | tr -d '\r')" 2>/dev/null)/starwars-vector.exe
WINDOWS_EXE_NATIVE := $(shell wslpath -w "$(WINDOWS_EXE)" 2>/dev/null)
WINDOWS_RUN_ARGS ?=

.PHONY: test test-unit build-windows run-windows

# Complete suite, including Ebitengine integration tests. The wrapper supplies
# a virtual X11 display automatically on headless Linux systems.
test:
	GO="$(GO)" ./scripts/test.sh ./...

# Fast display-independent packages for normal edit/test cycles.
test-unit:
	GO="$(GO)" ./scripts/test-unit.sh

# Build the native Windows executable. Override WINDOWS_EXE to choose another
# Windows-mounted output location when necessary.
build-windows:
	@test -n "$(WINDOWS_EXE)"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "$(GO)" build -o "$(WINDOWS_EXE)" .

# Start the native executable without blocking make. Pass game arguments via
# WINDOWS_RUN_ARGS, for example: make run-windows WINDOWS_RUN_ARGS='-profile cadet'
run-windows: build-windows
	cmd.exe /C start "" "$(WINDOWS_EXE_NATIVE)" $(WINDOWS_RUN_ARGS)
