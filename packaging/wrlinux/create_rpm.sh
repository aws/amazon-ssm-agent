#!/usr/bin/env bash
set -e

# Build and package Amazon SSM Agent as RPM for Wind River Linux (SysVinit, x86_64)
#
# Usage:
#   ./packaging/wrlinux/create_rpm.sh
#
# Prerequisites:
#   - Go 1.24+ installed
#   - rpmbuild installed (brew install rpm on macOS)
#
# Output:
#   bin/linux_amd64/amazon-ssm-agent.rpm

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_SPACE="$(cd "${SCRIPT_DIR}/../.." && pwd)"

ARCH="amd64"
TARGET="x86_64"
FOLDER="linux_${ARCH}"
VERSION=$(cat "${GO_SPACE}/VERSION")

echo "============================================================"
echo "Building Amazon SSM Agent ${VERSION} for Wind River Linux"
echo "Architecture: ${TARGET} (${ARCH})"
echo "============================================================"

# ---------------------------------------------------------------
# Step 1: Build binaries (cross-compile for linux/amd64)
# ---------------------------------------------------------------
echo ""
echo "[1/3] Building Go binaries..."

export GOOS=linux
export GOARCH=${ARCH}
export CGO_ENABLED=0

OUTDIR="${GO_SPACE}/bin/${FOLDER}"
mkdir -p "${OUTDIR}"

GO_FLAGS="-ldflags -s -w -trimpath"

cd "${GO_SPACE}"

echo "  -> amazon-ssm-agent"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/amazon-ssm-agent" \
  core/agent.go core/agent_unix.go core/agent_parser.go

echo "  -> ssm-agent-worker"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/ssm-agent-worker" \
  agent/agent.go agent/agent_unix.go agent/agent_parser.go

echo "  -> ssm-document-worker"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/ssm-document-worker" \
  agent/framework/processor/executer/outofproc/worker/main.go

echo "  -> ssm-session-worker"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/ssm-session-worker" \
  agent/framework/processor/executer/outofproc/sessionworker/main.go

echo "  -> ssm-session-logger"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/ssm-session-logger" \
  agent/session/logging/main.go

echo "  -> ssm-cli"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/ssm-cli" \
  agent/cli-main/cli-main.go

echo "  -> updater"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/updater" \
  agent/update/updater/updater.go agent/update/updater/updater_unix.go

echo "  -> ssm-setup-cli"
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "-s -w" -trimpath \
  -o "${OUTDIR}/ssm-setup-cli" \
  agent/setupcli/setupcli.go

echo "  Binaries built in ${OUTDIR}/"

# ---------------------------------------------------------------
# Step 2: Prepare rpmbuild workspace
# ---------------------------------------------------------------
echo ""
echo "[2/3] Preparing RPM build workspace..."

STAGEDIR="${GO_SPACE}/bin/${FOLDER}/staging"
RPM_TOPDIR="${GO_SPACE}/bin/${FOLDER}/rpmbuild"

rm -rf "${STAGEDIR}" "${RPM_TOPDIR}"

mkdir -p "${RPM_TOPDIR}"/{SPECS,SOURCES,BUILD,RPMS,SRPMS}
mkdir -p "${STAGEDIR}/usr/bin"
mkdir -p "${STAGEDIR}/etc/amazon/ssm"
mkdir -p "${STAGEDIR}/etc/init.d"
mkdir -p "${STAGEDIR}/var/lib/amazon/ssm"
mkdir -p "${STAGEDIR}/var/log/amazon/ssm"

# Copy binaries
cp "${OUTDIR}/amazon-ssm-agent"     "${STAGEDIR}/usr/bin/"
cp "${OUTDIR}/ssm-agent-worker"      "${STAGEDIR}/usr/bin/"
cp "${OUTDIR}/ssm-document-worker"   "${STAGEDIR}/usr/bin/"
cp "${OUTDIR}/ssm-session-worker"    "${STAGEDIR}/usr/bin/"
cp "${OUTDIR}/ssm-session-logger"    "${STAGEDIR}/usr/bin/"
cp "${OUTDIR}/ssm-cli"              "${STAGEDIR}/usr/bin/"

# Copy config files
cp "${GO_SPACE}/amazon-ssm-agent.json.template" "${STAGEDIR}/etc/amazon/ssm/"
cp "${GO_SPACE}/seelog_unix.xml"                 "${STAGEDIR}/etc/amazon/ssm/seelog.xml.template"
cp "${GO_SPACE}/README.md"                       "${STAGEDIR}/etc/amazon/ssm/"
cp "${GO_SPACE}/RELEASENOTES.md"                 "${STAGEDIR}/etc/amazon/ssm/"
cp "${GO_SPACE}/NOTICE.md"                       "${STAGEDIR}/etc/amazon/ssm/"

# Copy SysVinit init script
cp "${SCRIPT_DIR}/amazon-ssm-agent.init"         "${STAGEDIR}/etc/init.d/amazon-ssm-agent"
chmod 755 "${STAGEDIR}/etc/init.d/amazon-ssm-agent"

# ---------------------------------------------------------------
# Step 3: Build RPM
# ---------------------------------------------------------------
echo ""
echo "[3/3] Building RPM package..."

SPEC_FILE="${SCRIPT_DIR}/amazon-ssm-agent.spec"

# Create minimal fileattrs dir to suppress macOS rpmbuild file classification errors
EMPTY_FILEATTRS="${RPM_TOPDIR}/fileattrs"
mkdir -p "${EMPTY_FILEATTRS}"
echo '%__none_provides %{nil}' > "${EMPTY_FILEATTRS}/none.attr"

rpmbuild -bb \
  --target "${TARGET}-linux" \
  --define "_version ${VERSION}" \
  --define "_topdir ${RPM_TOPDIR}" \
  --define "_stagedir ${STAGEDIR}" \
  --define "_prefix /usr" \
  --define "_sysconfdir /etc" \
  --define "_localstatedir /var" \
  --define "_tmppath /tmp" \
  --define "_fileattrsdir ${EMPTY_FILEATTRS}" \
  "${SPEC_FILE}"

# Copy RPM to output directory
RPM_FILE=$(find "${RPM_TOPDIR}/RPMS/${TARGET}/" -name "*.rpm" -type f | head -1)

if [ -z "${RPM_FILE}" ]; then
  echo "ERROR: RPM file not found!"
  exit 1
fi

cp "${RPM_FILE}" "${OUTDIR}/amazon-ssm-agent-${VERSION}.wrlinux.${TARGET}.rpm"

echo ""
echo "============================================================"
echo "SUCCESS!"
echo ""
echo "RPM: ${OUTDIR}/amazon-ssm-agent-${VERSION}.wrlinux.${TARGET}.rpm"
echo ""
echo "Install on Wind River Linux:"
echo "  scp ${OUTDIR}/amazon-ssm-agent-${VERSION}.wrlinux.${TARGET}.rpm root@<host>:/tmp/"
echo "  ssh root@<host> rpm -i /tmp/amazon-ssm-agent-${VERSION}.wrlinux.${TARGET}.rpm"
echo ""
echo "The RPM will automatically:"
echo "  - Install binaries to /usr/bin/"
echo "  - Install config to /etc/amazon/ssm/"
echo "  - Install SysVinit init script to /etc/init.d/"
echo "  - Create /var/lib/amazon/ssm/ and /var/log/amazon/ssm/"
echo "  - Enable and start the agent"
echo "============================================================"
