#!/usr/bin/env bash
set -e

repo_root=$(git rev-parse --show-toplevel)
target_dir="${repo_root}/.invokex/runtimes/alpine"
rm -rf ${target_dir}
mkdir -p ${target_dir}

# Work from a temporary directory to avoid polluting the user's working direcotyr.
tmp_working_dir=$(mktemp -d)
trap "sudo rm -rf ${tmp_working_dir}" EXIT
pushd ${tmp_working_dir} > /dev/null

# TODO: might want to introduce version pinning here to prevent new releases from breaking InvokeX.
ARCH="$(uname -m)"
release_url="https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/${ARCH}"

# Grab the manifest and figure out which release is the latest release.
wget -nc "${release_url}/latest-releases.yaml"
rootFsName=$(yq '.. | select(has("flavor")) | select(.flavor == "alpine-minirootfs") | .file' ./latest-releases.yaml)
osName=$(yq '.. | select(has("flavor")) | select(.flavor == "alpine-netboot") | .file' ./latest-releases.yaml)

# Download and configure rootFs.
wget "${release_url}/${rootFsName}"
tar -xzf ${rootFsName} --one-top-level
rootFsName=$(ls -d */ | grep "alpine-minirootfs")
rootFsName=${rootFsName%/}
# TODO: configure rootFs here
sudo chown -R root:root ${rootFsName}
truncate -s 16MB "${rootFsName}.ext4"
sudo mkfs.ext4 -d ${rootFsName} -F "${rootFsName}.ext4"

# Download Alpine Linux distribution.
wget "${release_url}/${osName}"
tar -xzf ${osName} --one-top-level
osName=$(ls -d */ | grep "alpine-netboot")
osName=${osName%/}

# Alpine Linux is distributed with a compressed Linux kernel: vmlinuz-virt.
# To make this work with Firecracker, we need to uncompress it into vmlinux-virt.
wget https://raw.githubusercontent.com/torvalds/linux/refs/heads/master/scripts/extract-vmlinux
chmod +x ./extract-vmlinux
./extract-vmlinux "${osName}/boot/vmlinuz-virt" > vmlinux

# All of this is needed by Firecracker.
mv "./${rootFsName}.ext4" "${target_dir}/rootfs.ext4"
mv ./vmlinux "${target_dir}/vmlinux"

popd > /dev/null
