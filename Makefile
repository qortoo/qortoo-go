# This package links libqortoo_ffi.a via cgo and carries no default include/library
# search path (see cgo.go): CGO_CFLAGS/CGO_LDFLAGS must point at a staged qortoo-ffi
# native SDK. There is no published native SDK release yet, so a qortoo-rs checkout is
# currently the only source.
#
# If CGO_CFLAGS/CGO_LDFLAGS are not already set, native-sdk below derives them from a
# sibling qortoo-rs checkout (../qortoo-rs) and stages its native SDK automatically.
# Override the checkout location with QORTOO_RS_DIR=/path/to/qortoo-rs, or export
# CGO_CFLAGS/CGO_LDFLAGS yourself to skip auto-staging entirely (e.g. once a native SDK
# is installed some other way).
QORTOO_RS_DIR ?= ../qortoo-rs
NATIVE_SDK_DIR := $(QORTOO_RS_DIR)/target/native-sdk/debug

# Captured before the defaults below are applied, so native-sdk can tell whether the
# caller already provided CGO_CFLAGS/CGO_LDFLAGS.
CGO_FLAGS_PROVIDED := $(filter-out undefined,$(origin CGO_CFLAGS))

ifeq ($(origin CGO_CFLAGS),undefined)
export CGO_CFLAGS := -I$(abspath $(NATIVE_SDK_DIR)/include)
endif
ifeq ($(origin CGO_LDFLAGS),undefined)
export CGO_LDFLAGS := -L$(abspath $(NATIVE_SDK_DIR)/lib)
endif

.PHONY: native-sdk
native-sdk:
ifeq ($(CGO_FLAGS_PROVIDED),)
	@if [ ! -d "$(QORTOO_RS_DIR)" ]; then \
		echo "error: $(QORTOO_RS_DIR) not found" >&2; \
		echo "       set QORTOO_RS_DIR=/path/to/qortoo-rs, or export CGO_CFLAGS/CGO_LDFLAGS yourself (see README.md)" >&2; \
		exit 1; \
	fi
	$(MAKE) -C $(QORTOO_RS_DIR) native-sdk-stage
endif

.PHONY: test
test: native-sdk
	go vet ./...
	go test -race ./...

.PHONY: bench
bench: native-sdk
	go test -run '^$$' -bench . -benchmem ./...
