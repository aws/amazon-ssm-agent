#!/usr/bin/env bash
# Verifies linux/arm64 agent binaries start on an ARMv8.0 CPU (no LSE atomics).
#
# Each binary is executed under qemu-user with -cpu cortex-a53, which does not
# implement FEAT_LSE. A binary containing unguarded ARMv8.1 atomics (casa, swpl,
# ldadd, ...) is killed with SIGILL (exit 132) before main runs.
# See https://github.com/aws/amazon-ssm-agent/issues/683
#
# Usage: verify_arm64_baseline.sh <dir-with-linux_arm64-binaries>
set -uo pipefail

BIN_DIR=${1:?usage: $0 <binary-dir>}
QEMU=${QEMU:-$(command -v qemu-aarch64-static || command -v qemu-aarch64 || true)}
CPU=${QEMU_CPU_MODEL:-cortex-a53}
BINARIES="amazon-ssm-agent ssm-agent-worker updater ssm-cli ssm-document-worker ssm-session-logger ssm-session-worker ssm-setup-cli"

if [[ -z "$QEMU" ]]; then
    echo >&2 "Error: qemu-aarch64-static not found (apt-get install qemu-user-static)"
    exit 2
fi

failed=0
for name in $BINARIES; do
    bin="$BIN_DIR/$name"
    if [[ ! -f "$bin" ]]; then
        echo >&2 "Error: missing $bin"
        failed=1
        continue
    fi

    # Informational: count casa/swpl, the glibc lock pattern from #683.
    # Go code (GOARM64=v8.0) also contains LSE ops, but behind a runtime CPU
    # check, so this count is a hint only; the qemu run below is the gate.
    if command -v objdump >/dev/null; then
        lse=$(objdump -d "$bin" 2>/dev/null | grep -cE $'\t(casa|swpl)\t')
        echo "$name: $lse casa/swpl instructions"
    fi

    # -version for the agent; anything else just needs to get past startup.
    # Only SIGILL (132) is a failure: other exit codes or a timeout (124)
    # mean the process reached Go code.
    arg="-h"
    [[ "$name" == "amazon-ssm-agent" ]] && arg="-version"
    timeout 30 "$QEMU" -cpu "$CPU" "$bin" "$arg" >/dev/null 2>&1
    rc=$?
    if [[ $rc -eq 132 ]]; then
        echo >&2 "FAIL: $name SIGILL on $CPU (ARMv8.0); binary requires ARMv8.1+ atomics"
        failed=1
    else
        echo "OK:   $name starts on $CPU (exit $rc)"
    fi
done

exit $failed
