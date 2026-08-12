#!/bin/sh
set -eu

usage() {
    echo "usage: $0 --qortoo-go <path> --qortoo-rs <path> --output <new-directory>" >&2
    exit 2
}

qortoo_go=
qortoo_rs=
output_dir=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --qortoo-go)
            [ "$#" -ge 2 ] || usage
            qortoo_go=$2
            shift 2
            ;;
        --qortoo-rs)
            [ "$#" -ge 2 ] || usage
            qortoo_rs=$2
            shift 2
            ;;
        --output)
            [ "$#" -ge 2 ] || usage
            output_dir=$2
            shift 2
            ;;
        *) usage ;;
    esac
done

[ -n "$qortoo_go" ] && [ -n "$qortoo_rs" ] && [ -n "$output_dir" ] || usage
qortoo_go=$(cd "$qortoo_go" && pwd -P)
qortoo_rs=$(cd "$qortoo_rs" && pwd -P)

if [ -e "$output_dir" ]; then
    echo "error: output path already exists: $output_dir" >&2
    exit 1
fi

"$qortoo_go/scripts/check-benchmark-contract.sh" \
    --qortoo-go "$qortoo_go" \
    --qortoo-rs "$qortoo_rs"

make -C "$qortoo_rs" native-sdk-stage NATIVE_SDK_PROFILE=release
native_sdk=$qortoo_rs/target/native-sdk/release

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/qortoo-benchmark.XXXXXX")
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

cargo bench --manifest-path "$qortoo_rs/Cargo.toml" --bench qortoo_bench \
    > "$tmp_dir/rust.txt"
"$qortoo_go/scripts/run-go-benchmarks.sh" \
    --qortoo-go "$qortoo_go" \
    --native-sdk "$native_sdk" \
    > "$tmp_dir/go.txt"

{
    grep -E '^(goos|goarch|pkg):' "$tmp_dir/rust.txt"
    grep '^Benchmark' "$tmp_dir/rust.txt"
    grep '^Benchmark' "$tmp_dir/go.txt" | sed -E 's#(/impl=go)-[0-9]+#\1#'
} > "$tmp_dir/benchmarks.txt"

if ! grep -q '/impl=rust' "$tmp_dir/benchmarks.txt" || \
   ! grep -q '/impl=go' "$tmp_dir/benchmarks.txt"; then
    echo "error: the run did not produce both Rust and Go benchmark samples" >&2
    exit 1
fi

sdk_version=$(sed -n 's/.*"sdk_version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$native_sdk/manifest.json")
abi_major=$(sed -n 's/.*"abi_version_major"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$native_sdk/manifest.json")
abi_minor=$(sed -n 's/.*"abi_version_minor"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$native_sdk/manifest.json")
sdk_target=$(sed -n 's/.*"target"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$native_sdk/manifest.json")
testbed=$(hostname -s \
    | tr '[:upper:]' '[:lower:]' \
    | tr -c 'a-z0-9-' '-' \
    | sed 's/^-*//; s/-*$//')
if [ -n "$(git -C "$qortoo_go" status --porcelain)" ]; then
    qortoo_go_dirty=true
else
    qortoo_go_dirty=false
fi
if [ -n "$(git -C "$qortoo_rs" status --porcelain)" ]; then
    qortoo_rs_dirty=true
else
    qortoo_rs_dirty=false
fi

{
    echo "recorded_at_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "qortoo_go_commit=$(git -C "$qortoo_go" rev-parse HEAD)"
    echo "qortoo_go_branch=$(git -C "$qortoo_go" branch --show-current)"
    echo "qortoo_go_dirty=$qortoo_go_dirty"
    echo "qortoo_rs_commit=$(git -C "$qortoo_rs" rev-parse HEAD)"
    echo "qortoo_rs_branch=$(git -C "$qortoo_rs" branch --show-current)"
    echo "qortoo_rs_dirty=$qortoo_rs_dirty"
    echo "sdk_version=$sdk_version"
    echo "abi_version=$abi_major.$abi_minor"
    echo "sdk_target=$sdk_target"
    echo "hostname=$(hostname)"
    echo "testbed=$testbed"
    echo "os=$(uname -s)"
    echo "architecture=$(uname -m)"
    echo "host=$(uname -a)"
    echo "rustc=$(rustc --version)"
    echo "go=$(go version)"
} > "$tmp_dir/metadata.txt"

if command -v benchstat >/dev/null 2>&1; then
    benchstat -col /impl -ignore pkg,cpu "$tmp_dir/benchmarks.txt" \
        > "$tmp_dir/benchstat.txt"
fi

mkdir -p "$output_dir"
mv "$tmp_dir/benchmarks.txt" "$tmp_dir/metadata.txt" "$output_dir/"
if [ -f "$tmp_dir/benchstat.txt" ]; then
    mv "$tmp_dir/benchstat.txt" "$output_dir/"
fi

echo "saved benchmark run: $output_dir"
if [ -f "$output_dir/benchstat.txt" ]; then
    cat "$output_dir/benchstat.txt"
else
    echo "benchstat is not installed; raw samples are in $output_dir/benchmarks.txt"
fi
