# Building Amazon SSM Agent for Wind River Linux LTS

## Target

- **OS**: Wind River Linux LTS 22.33 Update 16 (`wrlinux`, VERSION_ID=10.22.33.16)
- **Architecture**: x86_64 (amd64)
- **Init system**: systemd

## Prerequisites

### On the build host (macOS / standard Linux)

- **Go 1.24+** (go.mod specifies 1.25, Docker uses 1.24)
- **make**
- No C compiler needed if using the static build (CGO_ENABLED=0)

### On the Wind River Linux target (runtime)

- **systemd** — service management
- **`/etc/machine-id`** — provided by systemd, used for instance fingerprint
- **`/usr/sbin/dmidecode`** — hardware fingerprint (optional, agent continues without it)
- **`/bin/hostname`** — FQDN detection
- **`rpm`** — required for self-update install/uninstall scripts
- **Network access** to AWS SSM endpoints

## Source Code Changes

The following files were modified to add Wind River Linux platform recognition:

| File | Change |
|------|--------|
| `agent/plugins/configurepackage/envdetect/constants/constants.go` | Added `PlatformWindRiver = "wrlinux"` constant |
| `agent/plugins/configurepackage/envdetect/osdetect/osdetect_unix.go` | Added `wrlinux` to OS-release ID mapping; mapped to `rhel` family (RPM-based) |
| `agent/updateutil/updateconstants/constants.go` | Added `PlatformWindRiver = "wind river"` constant |
| `agent/updateutil/updateinfo/updateinfo.go` | Added WR Linux to platform detection chain (uses `linux` download path); added to `possiblyUsingSystemD` map |

### Why these changes are needed

Without them, the agent reads `/etc/os-release` and gets `ID=wrlinux` / `NAME="Wind River Linux LTS"`. Neither value matches any known platform in the codebase, causing:

1. Platform detection falls through to the Windows/Nano path (incorrect)
2. Package manager detection fails (`configurePackage` plugin unusable)
3. Self-update downloads the wrong artifact

## Build Instructions

### Option A: Static Build — CGO_ENABLED=0 (Recommended)

Produces fully static Go binaries with **zero glibc dependency**. Best for embedded/minimal WR Linux images.

```bash
cd /path/to/amazon-ssm-agent

# Clean previous builds
rm -rf bin/linux_amd64

# Build all 8 binaries
make copy-src pre-build
cd build/private/src/github.com/aws/amazon-ssm-agent

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/amazon-ssm-agent -v \
  core/agent.go core/agent_unix.go core/agent_parser.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/ssm-agent-worker -v \
  agent/agent.go agent/agent_unix.go agent/agent_parser.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/updater -v \
  agent/update/updater/updater.go agent/update/updater/updater_unix.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/ssm-cli -v \
  agent/cli-main/cli-main.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/ssm-document-worker -v \
  agent/framework/processor/executer/outofproc/worker/main.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/ssm-session-logger -v \
  agent/session/logging/main.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/ssm-session-worker -v \
  agent/framework/processor/executer/outofproc/sessionworker/main.go

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath \
  -o ../../../../../../bin/linux_amd64/ssm-setup-cli -v \
  agent/setupcli/setupcli.go

cd -
```

Or use the Makefile shortcut (overriding GO_BUILD):

```bash
make copy-src pre-build
GOOS=linux GOARCH=amd64 GO_BUILD="CGO_ENABLED=0 go build -ldflags '-s -w' -trimpath" \
  make build-any-amd64-linux
```

### Option B: Docker Build (Reproducible)

Uses the project's Dockerfile for a clean build environment:

```bash
docker build -t ssm-agent-build-image .
docker run -it --rm --name ssm-agent-build \
  -v $(pwd):/amazon-ssm-agent \
  ssm-agent-build-image \
  make build-linux
```

This produces PIE binaries (with CGO) linked against the container's glibc. Verify glibc compatibility with your WR Linux target.

### Option C: PIE Build with WR Linux SDK Cross-Compiler

If your security policy requires PIE + RELRO hardening:

```bash
# Point CC to the WR Linux SDK cross-compiler
export CC=x86_64-wrs-linux-gnu-gcc
export CGO_ENABLED=1

GOOS=linux GOARCH=amd64 \
  GO_BUILD='go build -ldflags "-s -w -extldflags=-Wl,-z,now,-z,relro,-z,defs" -buildmode=pie -trimpath' \
  make build-any-amd64-linux
```

## Build & Package as RPM (Recommended)

The simplest approach: one script builds everything and produces an installable RPM.

### Prerequisites (build host — macOS)

```bash
brew install go rpm
```

### Build the RPM

```bash
cd /path/to/amazon-ssm-agent
chmod +x packaging/wrlinux/create_rpm.sh
./packaging/wrlinux/create_rpm.sh
```

Output: `bin/linux_amd64/amazon-ssm-agent-3.3.0.0.wrlinux.x86_64.rpm`

### Deploy to Wind River Linux

```bash
# Copy RPM to target
scp bin/linux_amd64/amazon-ssm-agent-*.wrlinux.x86_64.rpm root@<host>:/tmp/

# Install (first time)
ssh root@<host> rpm -i /tmp/amazon-ssm-agent-*.wrlinux.x86_64.rpm

# Upgrade (subsequent updates)
ssh root@<host> rpm -U /tmp/amazon-ssm-agent-*.wrlinux.x86_64.rpm

# Uninstall
ssh root@<host> rpm -e amazon-ssm-agent
```

The RPM automatically:

- Installs binaries to `/usr/bin/`
- Installs config to `/etc/amazon/ssm/`
- Installs SysVinit init script to `/etc/init.d/amazon-ssm-agent`
- Creates `/var/lib/amazon/ssm/` and `/var/log/amazon/ssm/`
- Enables the service at boot via rc symlinks
- Starts the agent on install, restarts on upgrade, stops on uninstall
- Creates `/etc/machine-id` if missing (needed for instance fingerprint)

### Register as a managed instance (if not EC2)

```bash
/etc/init.d/amazon-ssm-agent stop
amazon-ssm-agent -register -code "<ACTIVATION_CODE>" -id "<ACTIVATION_ID>" -region "<REGION>"
/etc/init.d/amazon-ssm-agent start
```

### Manage the service

```bash
/etc/init.d/amazon-ssm-agent start
/etc/init.d/amazon-ssm-agent stop
/etc/init.d/amazon-ssm-agent restart
/etc/init.d/amazon-ssm-agent status
```

## Verification

```bash
# Check agent is running
/etc/init.d/amazon-ssm-agent status

# Check agent logs
tail -f /var/log/amazon/ssm/amazon-ssm-agent.log

# Check platform detection
ssm-cli get-diagnostics
```

The agent should detect the platform as "Wind River Linux LTS" with version "10.22.33.16".

## Yocto/BitBake Integration (Optional)

If building WR Linux images via Yocto, create a recipe:

```bitbake
SUMMARY = "Amazon SSM Agent"
LICENSE = "Apache-2.0"
LIC_FILES_CHKSUM = "file://LICENSE;md5=..."

SRC_URI = "file://amazon-ssm-agent-3.3.0.0.tar.gz"

DEPENDS = "go-native"

do_compile() {
    export GOOS=linux
    export GOARCH=amd64
    export CGO_ENABLED=0
    # Build commands from Option A above
}

do_install() {
    install -d ${D}${bindir}
    install -d ${D}${sysconfdir}/amazon/ssm
    install -d ${D}${localstatedir}/lib/amazon/ssm
    install -d ${D}${systemd_system_unitdir}

    install -m 0555 ${B}/bin/linux_amd64/amazon-ssm-agent ${D}${bindir}/
    install -m 0555 ${B}/bin/linux_amd64/ssm-agent-worker ${D}${bindir}/
    # ... remaining binaries ...

    install -m 0644 ${S}/packaging/linux/amazon-ssm-agent.service \
        ${D}${systemd_system_unitdir}/
    install -m 0644 ${S}/amazon-ssm-agent.json.template \
        ${D}${sysconfdir}/amazon/ssm/
    install -m 0644 ${S}/seelog_unix.xml \
        ${D}${sysconfdir}/amazon/ssm/seelog.xml.template
}

inherit update-rc.d
INITSCRIPT_NAME = "amazon-ssm-agent"
INITSCRIPT_PARAMS = "defaults"
```

## Known Limitations

- **No systemd**: WR Linux LTS 22 on EC2 uses SysVinit (`/proc/1/comm` = `init`). A SysVinit init script is provided at `packaging/wrlinux/amazon-ssm-agent.init`. The systemd service file is **not used**.
- **Self-update will fail**: The `Tools/src/update/linux/install.sh` script checks for upstart, then systemd. If neither is found it exits with code 124 ("unsupported platform"). This means the SSM Agent self-update mechanism will not work on WR Linux. **Manage updates manually** or patch `install.sh`/`uninstall.sh` to add a SysVinit code path.
- **Self-update packages**: The updater downloads `linux_amd64` packages from AWS S3. These are Amazon-published RPMs built for standard Linux distributions. They *may* work on WR Linux if glibc is compatible, but this is not guaranteed.
- **configurePackage plugin**: Mapped to `yum` package manager (rhel family). The target has `rpm` (4.17.1) but **not `yum`/`dnf`**. The `AWS-ConfigureAWSPackage` document may fail if it invokes `yum`. Direct `rpm` operations will work.
- **Serial port startup**: Writes to `/dev/ttyS0` or `/dev/hvc0` on EC2. Non-fatal if serial port is unavailable.
- **`/etc/machine-id`**: Used for instance fingerprinting. If not present (systemd normally creates this), create it manually: `dbus-uuidgen > /etc/machine-id` or check if `/var/lib/dbus/machine-id` exists instead (the agent checks both paths).
