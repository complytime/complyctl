# Cleanup native-platform testing

Use this runbook to collect the runtime evidence for evidence-cleanup
task 5.4. It tests the current checkout, including uncommitted and
untracked implementation files, on the selected operating system and
filesystem. No registry credentials or external provider binaries are
needed; lifecycle tests use their own provider fixtures.

## Choose where to run

| Environment | Coverage it can provide |
| --- | --- |
| Current arm64 Linux devcontainer, with test fixtures on `/dev/shm` | Linux arm64/tmpfs |
| Apple Silicon Mac terminal, outside the devcontainer | macOS arm64/APFS |
| Native Intel Mac terminal | macOS amd64/APFS |
| Native amd64 Linux host or hardware-virtualized VM | Linux amd64; filesystem depends on the test mount |
| Native arm64 Linux host or hardware-virtualized VM | Linux arm64; filesystem depends on the test mount |
| Matching-architecture Docker/Podman container | Linux coverage on its actual test filesystem and kernel |
| amd64 container emulated on an arm64 machine | Supplemental emulated Linux coverage, not native amd64 evidence |

For the full matrix, record Linux amd64 and arm64 on ext4, XFS, Btrfs,
and tmpfs, plus macOS amd64 and arm64 on local APFS. Keep separate
results for every combination. Existing results can be reused when the
tested code is unchanged; a new run is not evidence for a different
architecture or filesystem.

A container image supplies userspace tools, not a different kernel or
filesystem. Selecting a Fedora or Debian image does not select XFS or
ext4. Docker Desktop and Podman Machine on macOS run **Linux VMs**;
their containers cannot test the Darwin backend or APFS behavior.
Neither Rosetta, QEMU CPU emulation, nor `GOOS` cross-compilation
satisfies the native architecture requirement. Record the physical
machine/VM architecture separately: container `uname` output alone does
not prove that execution is native.

## 1. Create the common test runner

Run all shell examples in **Bash**, from the root of this checkout.
On macOS, enter `bash` first if your terminal uses zsh. Go 1.26.10,
a C compiler, and the platform's C development libraries are required
for the race detector. On macOS, install Xcode Command Line Tools with
`xcode-select --install` if they are absent. In the Fedora devcontainer,
install missing compiler packages with `sudo dnf install gcc glibc-devel`
(omit `sudo` if already root).

Create this temporary runner once per machine. The container examples
also use this file. It writes logs and compiled test binaries outside
the filesystem under test, so a `noexec` tmpfs mount remains usable.
Tests create and clean their own synthetic workspaces beneath `TMPDIR`.

```bash
set -euo pipefail
cat > /tmp/complyctl-native-tests.sh <<'BASH'
#!/usr/bin/env bash
set -euo pipefail

: "${TEST_ROOT:?Set TEST_ROOT to an existing, writable test directory}"
test -d "$TEST_ROOT"
test -w "$TEST_ROOT"
test -f go.mod
export GOTOOLCHAIN=go1.26.10
export TMPDIR="$TEST_ROOT"
RUN_DIR="${RUN_DIR:-$(mktemp -d /tmp/complyctl-native-results.XXXXXX)}"
export RUN_DIR
mkdir -p "$RUN_DIR"

{
    date -u
    uname -s -m
    id
    go version
    go env GOOS GOARCH
    git rev-parse HEAD
    git status --short
    printf 'TEST_ROOT=%s\nRUN_DIR=%s\n' "$TEST_ROOT" "$RUN_DIR"
    if [ "$(uname -s)" = Linux ]; then
        stat -f -c 'filesystem=%T' "$TEST_ROOT"
        df -T "$TEST_ROOT"
    else
        df -h "$TEST_ROOT"
    fi
} 2>&1 | tee "$RUN_DIR/environment.log"

for package in \
    internal/workspacecoord \
    internal/evidencecleanup \
    cmd/complyctl/cli \
    pkg/provider
do
    name="${package##*/}"
    CGO_ENABLED=1 GOTMPDIR="$RUN_DIR" go test -mod=vendor -race -c \
        -o "$RUN_DIR/$name.test" "./$package" \
        2>&1 | tee "$RUN_DIR/$name-build.log"
    TMPDIR="$TEST_ROOT" GOTMPDIR="$TEST_ROOT" \
        "$RUN_DIR/$name.test" -test.v -test.count=1 -test.timeout=10m \
        2>&1 | tee "$RUN_DIR/$name-race.log"
done

CGO_ENABLED=0 GOTMPDIR="$RUN_DIR" go test -mod=vendor -c \
    -o "$RUN_DIR/evidencecleanup-nocgo.test" ./internal/evidencecleanup \
    2>&1 | tee "$RUN_DIR/evidencecleanup-nocgo-build.log"
TMPDIR="$TEST_ROOT" GOTMPDIR="$TEST_ROOT" "$RUN_DIR/evidencecleanup-nocgo.test" \
    -test.v -test.count=1 -test.timeout=10m \
    2>&1 | tee "$RUN_DIR/evidencecleanup-nocgo.log"

printf '\nCompleted. Keep results from: %s\n' "$RUN_DIR"
BASH
bash -n /tmp/complyctl-native-tests.sh
```

`set -o pipefail` preserves a failing test's exit status through `tee`.
The runner stops at the first failed build or suite; missing later logs
mean those suites did not run. Do not count such a run as complete.
Go 1.26's `t.TempDir()` prefers `GOTMPDIR`, so the runner sets **both**
`TMPDIR` and `GOTMPDIR` to the test filesystem when executing test
binaries. Only compilation uses `RUN_DIR` for build temporary files.
Setting only `TMPDIR` while leaving `GOTMPDIR` on overlay is insufficient.

## 2. Current Linux devcontainer: tmpfs

Run inside the devcontainer, from this checkout. Confirm that `/dev/shm`
has enough free space for the test fixtures; compilation happens in
`RUN_DIR`, not `/dev/shm`.

```bash
df -h /dev/shm
TEST_ROOT="$(mktemp -d /dev/shm/complyctl-native.XXXXXX)"
export TEST_ROOT
unset RUN_DIR
bash /tmp/complyctl-native-tests.sh
```

The environment log should report `tmpfs`, and
`TestCheckCleanupSupport_LinuxNativeFilesystem` should log `0x1021994`.
Do not use the default overlay-backed `/tmp` for supported-cleanup
coverage. An overlay run can pass the refusal assertion while skipping
the cleanup-success cases.

Some Linux tests create private user and mount namespaces to verify
nested-mount refusal. A container runtime or host policy can deny those
operations. An `operation not permitted` failure is an environment
limitation, not a reason to skip the test and report success. Use the
disposable privileged-container procedure below or a Linux VM where
those facilities are available. Keep any unrelated devcontainer policy
unchanged.

## 3. Native macOS: APFS

Run in the Mac terminal, **outside** the devcontainer, using the same
checkout with its uncommitted files. Run as your normal user, not root.

```bash
TEST_ROOT="$(mktemp -d "$HOME/Library/Caches/complyctl-native.XXXXXX")"
export TEST_ROOT
unset RUN_DIR
df -h "$TEST_ROOT"
diskutil info /System/Volumes/Data
bash /tmp/complyctl-native-tests.sh
```

The `diskutil` command describes the standard macOS Data volume. If
your home directory is on another volume, use the device shown by `df`
with `diskutil info` instead. The test itself checks the actual temp
directory: look for `native filesystem type: "apfs"` in
`workspacecoord-race.log`. Record whether this was an Apple Silicon
arm64 Mac or an Intel amd64 Mac. Run the same procedure on an Intel Mac
to supply the missing Darwin amd64 evidence.

## 4. Docker or Podman: Linux tmpfs

The official `docker.io/library/golang:1.26.10-bookworm` image has Linux
amd64 and arm64 variants. The following local derivative adds filesystem
tools and a normal test user. It does not modify this repository.
Image availability was checked; the container procedures below have
not been executed in the current devcontainer because it has neither
Docker nor Podman installed. Keep results from your actual run rather
than treating these instructions as a completed matrix entry.

Run the engine commands from the machine that owns the checkout and
can bind-mount it. With Docker Desktop/Podman Machine, that normally
means the host terminal, not a nested devcontainer shell. Install and
start Docker Desktop or Podman Machine first when using macOS; on
Linux, use a running Docker service or an available Podman engine.

```bash
export ENGINE=docker
# For Podman instead: export ENGINE=podman
"$ENGINE" info
REPO="$(pwd -P)"
export REPO
test -f "$REPO/go.mod"
test -f /tmp/complyctl-native-tests.sh
CONTAINER_FILES="$(mktemp -d "$HOME/complyctl-container-files.XXXXXX")"
export CONTAINER_FILES
install -m 0644 /tmp/complyctl-native-tests.sh "$CONTAINER_FILES/native-tests.sh"
export IMAGE_CONTEXT="$CONTAINER_FILES"

cat > "$IMAGE_CONTEXT/Dockerfile" <<'DOCKERFILE'
FROM docker.io/library/golang:1.26.10-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends \
    e2fsprogs xfsprogs btrfs-progs util-linux gcc libc6-dev git \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 1000 --create-home --shell /bin/bash runner
ENV GOTOOLCHAIN=go1.26.10
ENV HOME=/home/runner
ENV GOCACHE=/home/runner/.cache/go-build
ENV GOMODCACHE=/home/runner/go/pkg/mod
ENV GOTELEMETRY=off
WORKDIR /src
DOCKERFILE

"$ENGINE" build -t complyctl-native-test:go1.26.10 "$IMAGE_CONTEXT"
CONTAINER_RESULTS="$(mktemp -d "$HOME/complyctl-container-results.XXXXXX")"
export CONTAINER_RESULTS
# This fresh scratch directory must be writable by container UID 1000.
chmod 0777 "$CONTAINER_RESULTS"

"$ENGINE" run --rm --privileged \
    --user 1000:1000 --security-opt label=disable \
    --mount "type=bind,src=$REPO,dst=/src,readonly" \
    --mount "type=bind,src=$CONTAINER_FILES/native-tests.sh,dst=/run-native-tests.sh,readonly" \
    --mount "type=bind,src=$CONTAINER_RESULTS,dst=/results" \
    --tmpfs /testfs:rw,exec,size=2g,mode=1777 \
    -e TEST_ROOT=/testfs -e RUN_DIR=/results/tmpfs \
    complyctl-native-test:go1.26.10 \
    bash -c 'git config --global --add safe.directory /src; bash /run-native-tests.sh'

printf 'Host results directory: %s\n' "$CONTAINER_RESULTS"
```

The source bind mount includes the current working files, including
untracked implementation and vendored files; it is read-only. A clone,
`git archive`, or an old release image would omit those uncommitted
changes. The test fixtures live on Linux tmpfs, not the source bind
mount. No host HOME, credentials, or provider directory is mounted.

The engine must share `REPO`, `CONTAINER_FILES`, and `CONTAINER_RESULTS`
with its Linux VM. The latter two are under your home directory, which
Podman Machine normally shares. If the checkout is outside a shared
directory, place a full working copy under a shared directory or configure
that path in the engine's file-sharing settings before running. Do not
substitute a Git-only archive that drops untracked files.

The image build selects the engine's default architecture. To obtain
native amd64 evidence, run this on a native amd64 Linux machine/VM or
an Intel Mac's amd64 Linux VM. On Apple Silicon, this ordinarily tests
Linux arm64. `--platform=linux/amd64` on Apple Silicon may enable
emulation; it cannot turn that result into native amd64 evidence.

`--privileged` permits the nested user/mount namespace tests. Use a
disposable test machine or VM: privileged containers have broad access
to the engine's Linux host, even though the source mount is read-only.
For rootless Podman, privileges remain bounded by the outer user
namespace; if mount or loop-device operations fail, use a disposable
rootful Podman machine/VM. The examples disable SELinux container
label separation for this run instead of relabeling the checkout.
Other host restrictions, including AppArmor user-namespace policy,
can still prevent these tests. Do not change CI gates to hide them.

## 5. Docker or Podman: ext4, XFS and Btrfs

Reuse the image, runner, `ENGINE`, `REPO`, and `CONTAINER_RESULTS` from
section 4. This procedure creates fresh **regular image files**, formats
only those files, and mounts them inside a disposable container. It
does not accept or format an existing block device. Each image consumes
up to 2 GiB, with one image mounted at a time.

These filesystems are Linux-kernel features. The engine's host/VM must
support loop devices, ext4, XFS, Btrfs, and nested user/mount namespaces.
Installing `xfsprogs` or `btrfs-progs` alone does not add kernel support.
Docker Desktop/Podman Machine kernels may lack a filesystem or deny
loop-device operations. If so, use a native Linux test VM with the
required kernel support; record the failed environment attempt rather
than counting that filesystem as verified.

Create the temporary mount driver:

```bash
cat > /tmp/complyctl-filesystem-matrix.sh <<'BASH'
#!/usr/bin/env bash
set -euo pipefail
scratch="$(mktemp -d /tmp/complyctl-filesystems.XXXXXX)"
mountpoint="$scratch/mounted"
mkdir "$mountpoint"
mounted=0
cleanup() {
    if [ "$mounted" = 1 ]; then
        umount "$mountpoint" || return 1
    fi
    rm -rf -- "$scratch"
}
trap cleanup EXIT

for filesystem in ext4 xfs btrfs; do
    image="$scratch/$filesystem.img"
    truncate -s 2G "$image"
    case "$filesystem" in
        ext4) mkfs.ext4 -q "$image" ;;
        xfs) mkfs.xfs -q "$image" ;;
        btrfs) mkfs.btrfs -q "$image" ;;
    esac
    mount -t "$filesystem" -o loop "$image" "$mountpoint"
    mounted=1
    mkdir "$mountpoint/tests"
    chown runner:runner "$mountpoint/tests"
    # Mount as root; execute tests as a normal user so permission tests run.
    runuser -u runner -- env \
        TEST_ROOT="$mountpoint/tests" RUN_DIR="/results/$filesystem" \
        bash /run-native-tests.sh
    umount "$mountpoint"
    mounted=0
    rm -- "$image"
done
BASH
bash -n /tmp/complyctl-filesystem-matrix.sh
install -m 0644 /tmp/complyctl-filesystem-matrix.sh "$CONTAINER_FILES/filesystem-matrix.sh"

"$ENGINE" run --rm --privileged --security-opt label=disable \
    --mount "type=bind,src=$REPO,dst=/src,readonly" \
    --mount "type=bind,src=$CONTAINER_FILES/native-tests.sh,dst=/run-native-tests.sh,readonly" \
    --mount "type=bind,src=$CONTAINER_FILES/filesystem-matrix.sh,dst=/run-matrix.sh,readonly" \
    --mount "type=bind,src=$CONTAINER_RESULTS,dst=/results" \
    complyctl-native-test:go1.26.10 \
    bash -c 'runuser -u runner -- git config --global --add safe.directory /src; bash /run-matrix.sh' \
    2>&1 | tee "$CONTAINER_RESULTS/filesystem-matrix.log"
```

Run with `set -o pipefail` in the calling Bash shell, as well as inside
the scripts. If a filesystem fails, the script stops before subsequent
filesystems. Keep `filesystem-matrix.log` and the completed per-filesystem
directories. Do not interpret absent directories as successful runs.
Run the procedure on both native Linux architectures to fill both rows
of filesystem evidence. To use already-mounted ext4/XFS/Btrfs on a
Linux host instead, set `TEST_ROOT` to a fresh writable directory there
and run section 1's common runner directly.

## 6. Decide whether a matrix entry is complete

Keep the following with each result directory:

- Physical host/VM architecture and whether CPU emulation was enabled.
- Container engine, image ID, and Linux VM/kernel details when applicable.
  Record `docker image inspect complyctl-native-test:go1.26.10` or the
  equivalent `podman image inspect` output.
- `environment.log`, four `*-race.log` files, and the CGO-disabled
  cleanup log. Keep build logs too, particularly on failure.
- The checkout revision and working-tree state used for the run. Keep
  the candidate files unchanged while gathering evidence across machines.

Require passing results for locking, preview, whole-group deletion,
preflight retention, runtime partial failures, interrupted activity,
and manual recovery. These suites contain the behavioral and subprocess
fixtures for those cases; do not perform manual cleanup on real evidence
to test them. Inspect every `SKIP` rather than relying only on exit zero.
For example, a case-insensitive-path test may legitimately skip on a
case-sensitive Linux filesystem, but skipped cleanup-success tests due
to overlay do not establish supported cleanup behavior. Root-only
permission skips also need a normal-user rerun.

Expected Linux native filesystem identifiers in the coordination log:

| Filesystem | Identifier |
| --- | --- |
| ext4 | `0xef53` |
| XFS | `0x58465342` |
| Btrfs | `0x9123683e` |
| tmpfs | `0x1021994` |

Only mark task 5.4 complete when all required matrix entries have actual
runtime evidence. Missing hardware, kernel support, permissions, or
filesystem mounts remain missing verification, not failed product
behavior and not a passing test result. No container image supplies a
substitute for native Intel macOS/APFS.

### Runbook verification

The common runner completed all four race suites and the CGO-disabled
cleanup suite on Linux arm64/tmpfs on 2026-11-25. The coordination test
reported `0x1021994`; the only skips were the two case-equivalent-path
subcases on case-sensitive tmpfs, in each cleanup run. All Bash examples
and embedded scripts passed Bash syntax checks and ShellCheck. The
container and native macOS procedures still require execution in their
respective environments; syntax validation is not runtime evidence.

Related: [cleanup scope and recovery](EVIDENCE.md),
[development test environment](TESTING_ENVIRONMENT.md), and
[the evidence-cleanup task list](../openspec/changes/evidence-cleanup/tasks.md).
