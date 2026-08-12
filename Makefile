# This package links libqortoo_ffi.a via cgo and carries no default include/library
# search path (see cgo.go): CGO_CFLAGS/CGO_LDFLAGS must point at a staged qortoo-ffi
# native SDK. There is no published native SDK release yet, so a qortoo-rs checkout is
# currently the only source.
#
# For tests, native-sdk stages a debug SDK from a sibling qortoo-rs checkout when the
# cgo flags are not already set. Override that location with
# QORTOO_RS_DIR=/path/to/qortoo-rs, or export both flags to use an installed SDK. The
# benchmark target always stages a release SDK from QORTOO_RS_DIR.
QORTOO_RS_DIR ?= ../qortoo-rs
NATIVE_SDK_PROFILE ?= debug
NATIVE_SDK_DIR = $(QORTOO_RS_DIR)/target/native-sdk/$(NATIVE_SDK_PROFILE)

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
	go test -race ./...

.PHONY: bench
bench: NATIVE_SDK_PROFILE = release
bench:
	@if [ ! -d "$(QORTOO_RS_DIR)" ]; then \
		echo "error: $(QORTOO_RS_DIR) not found" >&2; \
		echo "       set QORTOO_RS_DIR=/path/to/qortoo-rs (see docs/performance.md)" >&2; \
		exit 1; \
	fi
	$(MAKE) -C $(QORTOO_RS_DIR) native-sdk-stage NATIVE_SDK_PROFILE=release
	./scripts/run-go-benchmarks.sh \
		--qortoo-go "$(CURDIR)" \
		--native-sdk "$(abspath $(NATIVE_SDK_DIR))"

# Cross-language benchmark result storage and Bencher tracking. Each saved run is a
# directory containing raw samples, environment/commit metadata, and benchstat output.
BENCH_RESULTS ?= benchmarks/results
BENCH_HOST ?= $(shell hostname -s | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-' '-' | sed 's/^-*//; s/-*$$//')
BENCH_TIMESTAMP := $(shell date -u +%Y-%m-%dT%H%M%SZ)
BENCH_GO_SHA := $(shell git rev-parse --short HEAD)
BENCH_RS_SHA := $(shell git -C "$(QORTOO_RS_DIR)" rev-parse --short HEAD 2>/dev/null || echo unknown)
BENCH_RESULT_DIR ?= $(BENCH_RESULTS)/$(BENCH_TIMESTAMP)-go-$(BENCH_GO_SHA)-rs-$(BENCH_RS_SHA)-$(BENCH_HOST)
BENCH_LATEST = $(shell find "$(BENCH_RESULTS)" -mindepth 1 -maxdepth 1 -type d -print 2>/dev/null | sort -r | head -1)
BENCHER_PROJECT ?= qortoo-sync
BENCHER_TESTBED ?=
BENCHER_DRY_RUN ?= 0

.PHONY: bench-save
bench-save:
	./scripts/compare-benchmarks.sh \
		--qortoo-go "$(CURDIR)" \
		--qortoo-rs "$(abspath $(QORTOO_RS_DIR))" \
		--output "$(BENCH_RESULT_DIR)"

# RESULT is the preferred selector. FILE remains accepted for compatibility with the
# former qortoo-rs Makefile targets. Both can name a result directory or benchmarks.txt.
.PHONY: bench-overhead
bench-overhead:
	@result="$(or $(RESULT),$(FILE),$(BENCH_LATEST))"; \
	test -n "$$result" || { echo "error: no saved benchmark result found" >&2; exit 1; }; \
	./scripts/benchmark-result.sh overhead --result "$$result"

.PHONY: bench-compare
bench-compare:
	@base="$(BASE)"; new="$(or $(NEW),$(BENCH_LATEST))"; \
	test -n "$$base" || { echo "usage: make bench-compare BASE=<result> [NEW=<result>]" >&2; exit 1; }; \
	test -n "$$new" || { echo "error: no saved benchmark result found for NEW" >&2; exit 1; }; \
	./scripts/benchmark-result.sh compare --base "$$base" --new "$$new"

.PHONY: bench-upload
bench-upload:
	@result="$(or $(RESULT),$(FILE),$(BENCH_LATEST))"; \
	test -n "$$result" || { echo "error: no saved benchmark result found" >&2; exit 1; }; \
	BENCHER_PROJECT="$(BENCHER_PROJECT)" \
	BENCHER_TESTBED="$(BENCHER_TESTBED)" \
	BENCHER_DRY_RUN="$(BENCHER_DRY_RUN)" \
		./scripts/benchmark-result.sh upload --result "$$result"
