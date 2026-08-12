#!/bin/sh
set -eu

usage() {
    echo "usage: $0 --qortoo-go <path> --qortoo-rs <path>" >&2
    exit 2
}

qortoo_go=
qortoo_rs=
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
        *) usage ;;
    esac
done

[ -n "$qortoo_go" ] && [ -n "$qortoo_rs" ] || usage
qortoo_go=$(cd "$qortoo_go" && pwd -P)
qortoo_rs=$(cd "$qortoo_rs" && pwd -P)

contract=$qortoo_go/benchmarks/contract.tsv
go_harness=$qortoo_go/benchmark_test.go
rust_harness=$qortoo_rs/benches/qortoo_bench.rs
for required in "$contract" "$go_harness" "$rust_harness"; do
    if [ ! -f "$required" ]; then
        echo "error: required benchmark file not found: $required" >&2
        exit 1
    fi
done

tab=$(printf '\t')
contract_count=0
while IFS="$tab" read -r scenario budget rust_budget workload; do
    case "$scenario" in
        ''|'#'*) continue ;;
    esac
    contract_count=$((contract_count + 1))

    case "$budget" in
        ''|*[!0-9]*)
            echo "error: $scenario has a non-numeric budget in $contract" >&2
            exit 1
            ;;
    esac

    if ! grep -Eq "^func Benchmark${scenario}\\(b \\*testing\\.B\\)" "$go_harness"; then
        echo "error: Benchmark$scenario is missing from $go_harness" >&2
        exit 1
    fi

    if ! grep -Fq "scenario(\"$scenario\", $rust_budget," "$rust_harness"; then
        echo "error: Rust scenario $scenario does not use $rust_budget" >&2
        exit 1
    fi

    rust_value=$(sed -n "s/^const $rust_budget: u64 = \\([0-9_][0-9_]*\\);/\\1/p" "$rust_harness")
    rust_value=$(printf '%s' "$rust_value" | tr -d '_')
    if [ "$rust_value" != "$budget" ]; then
        echo "error: $scenario budget is $budget in the contract but $rust_budget is $rust_value" >&2
        exit 1
    fi
done < "$contract"

go_count=$(grep -Ec '^func Benchmark[A-Za-z0-9_]+\(b \*testing\.B\)' "$go_harness")
rust_count=$(grep -Ec 'scenario\("[A-Za-z0-9_]+", BUDGET_[A-Z0-9_]+,' "$rust_harness")
if [ "$go_count" -ne "$contract_count" ] || [ "$rust_count" -ne "$contract_count" ]; then
    echo "error: benchmark scenario count differs (contract=$contract_count go=$go_count rust=$rust_count)" >&2
    exit 1
fi

echo "benchmark contract matches: $contract_count scenarios"
