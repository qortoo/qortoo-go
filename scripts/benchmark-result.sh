#!/bin/sh
set -eu

usage() {
    cat >&2 <<EOF
usage:
  $0 overhead --result <result-directory-or-benchmark-file>
  $0 compare --base <result-directory-or-benchmark-file> --new <result-directory-or-benchmark-file>
  $0 upload --result <result-directory-or-benchmark-file> [--project <project>] [--testbed <testbed>] [--dry-run]
EOF
    exit 2
}

resolve_benchmark_file() {
    result=$1
    if [ -d "$result" ]; then
        result=$result/benchmarks.txt
    fi
    if [ ! -s "$result" ]; then
        echo "error: benchmark result not found or empty: $result" >&2
        exit 1
    fi
    printf '%s\n' "$result"
}

resolve_metadata_file() {
    result=$1
    if [ -d "$result" ]; then
        metadata=$result/metadata.txt
    else
        metadata=$(dirname "$result")/metadata.txt
    fi
    if [ -s "$metadata" ]; then
        printf '%s\n' "$metadata"
    fi
}

metadata_value() {
    key=$1
    metadata=$2
    [ -n "$metadata" ] || return 0
    sed -n "s/^${key}=//p" "$metadata" | head -1
}

require_benchstat() {
    if ! command -v benchstat >/dev/null 2>&1; then
        echo "error: benchstat is required; install it with:" >&2
        echo "       go install golang.org/x/perf/cmd/benchstat@latest" >&2
        exit 1
    fi
}

sanitize_testbed() {
    printf '%s' "$1" \
        | tr '[:upper:]' '[:lower:]' \
        | tr -c 'a-z0-9-' '-' \
        | sed 's/^-*//; s/-*$//'
}

command=${1:-}
[ -n "$command" ] || usage
shift

case "$command" in
    overhead)
        result=
        while [ "$#" -gt 0 ]; do
            case "$1" in
                --result)
                    [ "$#" -ge 2 ] || usage
                    result=$2
                    shift 2
                    ;;
                *) usage ;;
            esac
        done
        [ -n "$result" ] || usage
        require_benchstat
        benchmark_file=$(resolve_benchmark_file "$result")
        benchstat -col /impl -ignore pkg,cpu "$benchmark_file"
        ;;

    compare)
        base=
        new=
        while [ "$#" -gt 0 ]; do
            case "$1" in
                --base)
                    [ "$#" -ge 2 ] || usage
                    base=$2
                    shift 2
                    ;;
                --new)
                    [ "$#" -ge 2 ] || usage
                    new=$2
                    shift 2
                    ;;
                *) usage ;;
            esac
        done
        [ -n "$base" ] && [ -n "$new" ] || usage
        require_benchstat

        base_file=$(resolve_benchmark_file "$base")
        new_file=$(resolve_benchmark_file "$new")
        base_metadata=$(resolve_metadata_file "$base")
        new_metadata=$(resolve_metadata_file "$new")
        base_host=$(metadata_value hostname "$base_metadata")
        new_host=$(metadata_value hostname "$new_metadata")
        if [ -n "$base_host" ] && [ -n "$new_host" ] && [ "$base_host" != "$new_host" ]; then
            echo "error: benchmark results came from different hosts: $base_host and $new_host" >&2
            exit 1
        fi

        benchstat -ignore pkg,cpu "$base_file" "$new_file"
        ;;

    upload)
        result=
        project=${BENCHER_PROJECT:-qortoo-sync}
        testbed=${BENCHER_TESTBED:-}
        case ${BENCHER_DRY_RUN:-0} in
            1|true|yes) dry_run=true ;;
            *) dry_run=false ;;
        esac
        while [ "$#" -gt 0 ]; do
            case "$1" in
                --result)
                    [ "$#" -ge 2 ] || usage
                    result=$2
                    shift 2
                    ;;
                --project)
                    [ "$#" -ge 2 ] || usage
                    project=$2
                    shift 2
                    ;;
                --testbed)
                    [ "$#" -ge 2 ] || usage
                    testbed=$2
                    shift 2
                    ;;
                --dry-run)
                    dry_run=true
                    shift
                    ;;
                *) usage ;;
            esac
        done
        [ -n "$result" ] || usage
        if ! command -v bencher >/dev/null 2>&1; then
            echo "error: the Bencher CLI is not installed" >&2
            exit 1
        fi

        benchmark_file=$(resolve_benchmark_file "$result")
        metadata=$(resolve_metadata_file "$result")
        commit=$(metadata_value qortoo_go_commit "$metadata")
        branch=$(metadata_value qortoo_go_branch "$metadata")
        if [ -z "$testbed" ]; then
            testbed=$(metadata_value testbed "$metadata")
        fi
        if [ -z "$testbed" ]; then
            testbed=$(sanitize_testbed "$(metadata_value hostname "$metadata")")
        fi
        if [ -z "$testbed" ]; then
            testbed=$(sanitize_testbed "$(hostname -s)")
        fi

        if [ "$dry_run" = false ]; then
            credential=${BENCHER_API_KEY:-${BENCHER_API_TOKEN:-}}
            if [ -z "$credential" ]; then
                echo "error: set BENCHER_API_KEY to a Bencher project or user API key" >&2
                exit 1
            fi
            case "$credential" in
                bencher_*)
                    export BENCHER_API_KEY=$credential
                    unset BENCHER_API_TOKEN
                    ;;
                *)
                    export BENCHER_API_TOKEN=$credential
                    unset BENCHER_API_KEY
                    ;;
            esac
        fi

        set -- bencher run --project "$project" --testbed "$testbed" \
            --adapter go_bench --file "$benchmark_file"
        if [ -n "$commit" ]; then
            set -- "$@" --hash "$commit"
        fi
        if [ -n "$branch" ]; then
            set -- "$@" --branch "$branch"
        fi
        if [ "$dry_run" = true ]; then
            set -- "$@" --dry-run
        fi
        "$@"
        ;;

    *) usage ;;
esac
