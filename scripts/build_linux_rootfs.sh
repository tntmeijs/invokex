#!/usr/bin/env bash
set -e

debug=false

main() {
    if [ -z "$1" ]
    then
        echo "No runtime specified"
        exit 1
    fi

    if [ ! -z "$2" ] && [ "$2" == "debug" ]
    then
        echo "Script debugging enabled"
        debug=true
    fi

    local runtime_id="$1"
    local repo_root=$(git rev-parse --show-toplevel)
    local architecture="$(uname -m)"
    local target_dir="${repo_root}/.invokex/runtime"
    local working_dir=$(mktemp -d)
    local rootfs_size="512MB"

    trap "cleanup '${working_dir}'" EXIT

    build_base_rootfs_structure "${working_dir}" "${runtime_id}"
    rootfs_to_ext4 "${working_dir}" "${runtime_id}" "${architecture}" "${rootfs_size}" "${target_dir}"
}

cleanup() {
    local working_dir=$1

    if [ $debug == true ]
    then
        open "${working_dir}"
        wait_for_user
    fi

    sudo rm -rf "${working_dir}"
}

build_base_rootfs_structure() {
    local working_dir=$1
    local runtime_id=$2

    mkdir -p "${working_dir}/rootfs"
    pushd "${working_dir}/rootfs"

    # Userspace initialisation script.
    build_userspace_init "${repo_root}" "${working_dir}" "${runtime_id}"

    # Standard directory structure
    mkdir -p bin sbin etc dev proc sys tmp

    popd
}

build_userspace_init() {
    local repo_root=$1
    local working_dir=$2
    local runtime_id=$3

    pushd "${working_dir}/rootfs"

    # If the init binary panics, compile it with "-s" to include debug sysbols.
    # By default debug symbols are excluded from the binary to reduce its size.
    go build -C "${repo_root}/src/harness" -o "${working_dir}/rootfs/init" -ldflags "-s -X 'main.runtime=${runtime_id}'"
    chmod +x "./init"

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

wait_for_user() {
    read -p "press <enter> to resume the script"$'\n'
}

main "$@"
