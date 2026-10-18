# This package links libqortoo_ffi.a via cgo and carries no default include/library
# search path (see cgo.go): CGO_CFLAGS/CGO_LDFLAGS must point at a staged qortoo-ffi
# native SDK. There is no published native SDK release yet, so a qortoo-rs checkout is
# currently the only source.
#
# For tests, native-sdk stages a debug SDK from a sibling qortoo-rs checkout when the
# cgo flags are not already set. Override that location with
# QORTOO_RS_DIR=/path/to/qortoo-rs, or export both flags to use an installed SDK.
QORTOO_RS_DIR ?= ../qortoo-rs
NATIVE_SDK_PROFILE ?= debug
NATIVE_SDK_DIR = $(QORTOO_RS_DIR)/target/native-sdk/$(NATIVE_SDK_PROFILE)
GOLANGCI_LINT_MIN_VERSION ?= 2.12.2
COVERAGE_PROFILE ?= coverage.out
COVERAGE_HTML ?= coverage.html
GO_COVERAGE_MIN ?= 90

# Captured before the defaults below are applied, so native-sdk can tell whether the
# caller already provided CGO_CFLAGS/CGO_LDFLAGS.
CGO_FLAGS_PROVIDED := $(and $(filter-out undefined,$(origin CGO_CFLAGS)),$(filter-out undefined,$(origin CGO_LDFLAGS)))

ifeq ($(origin CGO_CFLAGS),undefined)
export CGO_CFLAGS = -I$(abspath $(NATIVE_SDK_DIR)/include)
endif
ifeq ($(origin CGO_LDFLAGS),undefined)
export CGO_LDFLAGS = -L$(abspath $(NATIVE_SDK_DIR)/lib)
endif

.PHONY: native-sdk
native-sdk:
ifeq ($(CGO_FLAGS_PROVIDED),)
	@if [ ! -d "$(QORTOO_RS_DIR)" ]; then \
		echo "error: $(QORTOO_RS_DIR) not found" >&2; \
		echo "       set QORTOO_RS_DIR=/path/to/qortoo-rs, or export CGO_CFLAGS/CGO_LDFLAGS yourself (see README.md)" >&2; \
		exit 1; \
	fi
	$(MAKE) -C $(QORTOO_RS_DIR) native-sdk-stage NATIVE_SDK_PROFILE=$(NATIVE_SDK_PROFILE)
endif

.PHONY: test
test: native-sdk
	go vet ./...
	# Rebuild and rerun against the native library, which Go's caches do not track.
	go test -a -race -count=1 ./...

.PHONY: fmt-check
fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "error: the following files are not gofmt-formatted:" >&2; \
		echo "$$unformatted" >&2; \
		exit 1; \
	fi

.PHONY: lint
lint: native-sdk fmt-check
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "error: golangci-lint >= $(GOLANGCI_LINT_MIN_VERSION) is required" >&2; \
		echo "       install the pinned CI version from https://golangci-lint.run/docs/welcome/install/" >&2; \
		exit 1; \
	}
	@lint_version="$$(golangci-lint version | sed -n 's/.*version \([0-9][^ ]*\).*/\1/p')"; \
	test -n "$$lint_version" || { echo "error: unable to determine golangci-lint version" >&2; exit 1; }; \
	awk -v got="$$lint_version" -v min="$(GOLANGCI_LINT_MIN_VERSION)" 'BEGIN { \
		sub(/^v/, "", got); sub(/^v/, "", min); split(got, g, "."); split(min, m, "."); \
		for (i = 1; i <= 3; i++) { \
			if ((g[i] + 0) > (m[i] + 0)) exit 0; \
			if ((g[i] + 0) < (m[i] + 0)) exit 1; \
		} \
		exit 0; \
	}' || { \
		echo "error: golangci-lint $$lint_version is older than required $(GOLANGCI_LINT_MIN_VERSION)" >&2; \
		exit 1; \
	}
	golangci-lint run ./...

.PHONY: coverage
coverage: native-sdk
	go test -a -race -count=1 -covermode=atomic -coverprofile="$(COVERAGE_PROFILE)" .
	go tool cover -func="$(COVERAGE_PROFILE)"
	go tool cover -html="$(COVERAGE_PROFILE)" -o="$(COVERAGE_HTML)"
	@total="$$(go tool cover -func="$(COVERAGE_PROFILE)" | awk '/^total:/ { gsub(/%/, "", $$3); print $$3 }')"; \
	test -n "$$total" || { echo "error: unable to read total Go coverage" >&2; exit 1; }; \
	awk -v total="$$total" -v minimum="$(GO_COVERAGE_MIN)" 'BEGIN { exit !((total + 0) >= (minimum + 0)) }' || { \
		echo "error: Go statement coverage $$total% is below $(GO_COVERAGE_MIN)%" >&2; \
		exit 1; \
	}; \
	echo "Go statement coverage $$total% meets the $(GO_COVERAGE_MIN)% minimum"
