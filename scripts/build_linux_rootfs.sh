#!/usr/bin/env bash
set -e

main() {
    if [ -z "$1" ]
    then
        echo "No runtime specified, supported runtimes:"
        echo "  - golang"
        exit 1
    fi

    local runtime_id="$1"
    local repo_root=$(git rev-parse --show-toplevel)
    local architecture="$(uname -m)"
    local target_dir="${repo_root}/.invokex/runtime"
    local working_dir=$(mktemp -d)
    local busybox_version="1.35.0"
    local busybox="${target_dir}/busybox"
    local rootfs_size="512MB"

    trap "cleanup '${working_dir}'" EXIT

    if [ ! -f "${busybox}" ]; then
        download_busybox "${busybox}" "${busybox_version}" "${architecture}"
    fi

    build_base_rootfs_structure "${working_dir}" "${busybox}" "${runtime_id}"
    rootfs_to_ext4 "${working_dir}" "${runtime_id}" "${architecture}" "${rootfs_size}" "${target_dir}"
}

cleanup() {
    local working_dir=$1

    # For debugging purposes
    open "${working_dir}"
    read -p "Waiting before nuking the working directory"

    sudo rm -rf "${working_dir}"
}

download_busybox() {
    local busybox=$1
    local version=$2
    local architecture=$3

    # Busybox will be used as a shell and provide common UNIX tools.
    wget -O "${busybox}" "https://busybox.net/downloads/binaries/${version}-${architecture}-linux-musl/busybox"
}

build_base_rootfs_structure() {
    local working_dir=$1
    local busybox=$2
    local runtime_id=$3

    mkdir -p "${working_dir}/rootfs"
    pushd "${working_dir}/rootfs"

    # Userspace initialisation script.
    build_userspace_init "${working_dir}" "${runtime_id}"

    # Standard directory structure
    mkdir -p bin sbin etc dev proc sys tmp

    # Add Busybox to the filesystem and create symlinks for required tools.
    cp "${busybox}" "bin/busybox"
    chmod +x bin/busybox
    ln -s /bin/busybox bin/sh
    ln -s /bin/busybox bin/ls
    ln -s /bin/busybox bin/mount

    popd
}

build_userspace_init() {
    local working_dir=$1
    local runtime_id=$2
    pushd "${working_dir}"

    # Create the init script until the PATH construction.
    cat <<EOF > "./rootfs/init"
#!/bin/sh
#
# Root filesystem init for "${runtime_id}" runtime.
#
# This is a special script that will initialize the system; the userspace entrypoint.
# Specifically, this will be PID 1 and it is what will set up userspace and start the actual system.
# This script will be placed in the rootfs so the Linux kernel can use it during its boot sequence.

# Mount kernel-provided virtual filesystems.
mount -t proc none /proc    # process list, memory info, cpu info, kernel parameters, etc
mount -t sysfs none /sys    # provides network interfaces, block devices, kernel subsystems, etc
mount -t devtmpfs none /dev # kernel-managed device nodes - disk access, console output, etc
mount -t tmpfs none /run    # RAM-backed temporary filesystem, for sockets, PID files, etc

# Basic environment setup.
export PATH=/bin:/sbin:/usr/bin:/usr/sbin

# TODO: change the entrypoint to whatever code was uploaded by the user and execute it.
# Start shell as PID 1 (replaces this wrapper).
exec /bin/sh
EOF

    chmod +x "./rootfs/init"

    popd
}

rootfs_to_ext4() {
    local working_dir=$1
    local runtime=$2
    local architecture=$3
    local size=$4
    local target_dir=$5
    local volume_name="rootfs_${runtime}_${architecture}.ext4"

    # Convert to ext4.
    sudo chown -R root:root "${working_dir}/rootfs"
    truncate -s "${size}" "${working_dir}/${volume_name}"
    sudo mkfs.ext4 -d "${working_dir}/rootfs" -F "${working_dir}/${volume_name}"

    # Put it where Firecracker can find it.
    mv "${working_dir}/${volume_name}" "${target_dir}/${volume_name}"
}

main "$@"
