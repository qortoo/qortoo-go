#!/bin/sh
set -eu

usage() {
    echo "usage: $0 --qortoo-go <path> --native-sdk <path>" >&2
    exit 2
}

qortoo_go=
native_sdk=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --qortoo-go)
            [ "$#" -ge 2 ] || usage
            qortoo_go=$2
            shift 2
            ;;
        --native-sdk)
            [ "$#" -ge 2 ] || usage
            native_sdk=$2
            shift 2
            ;;
        *) usage ;;
    esac
done

[ -n "$qortoo_go" ] && [ -n "$native_sdk" ] || usage
qortoo_go=$(cd "$qortoo_go" && pwd -P)
native_sdk=$(cd "$native_sdk" && pwd -P)

for required in \
    "$qortoo_go/benchmarks/contract.tsv" \
    "$native_sdk/include/qortoo.h" \
    "$native_sdk/lib/libqortoo_ffi.a" \
    "$native_sdk/manifest.json"; do
    if [ ! -f "$required" ]; then
        echo "error: required benchmark input not found: $required" >&2
        exit 1
    fi
done

if ! grep -Eq '"profile"[[:space:]]*:[[:space:]]*"release"' "$native_sdk/manifest.json"; then
    echo "error: Go benchmarks require a release native SDK: $native_sdk" >&2
    exit 1
fi

tab=$(printf '\t')
while IFS="$tab" read -r scenario budget rust_budget workload; do
    case "$scenario" in
        ''|'#'*) continue ;;
    esac
    (
        cd "$qortoo_go"
        CGO_CFLAGS="-I$native_sdk/include" \
        CGO_LDFLAGS="-L$native_sdk/lib" \
            go test -run '^$' -bench "^Benchmark${scenario}$" -benchmem \
                -benchtime "${budget}x" -count 10 .
    )
done < "$qortoo_go/benchmarks/contract.tsv"
