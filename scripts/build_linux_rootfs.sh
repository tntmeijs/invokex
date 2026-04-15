#!/usr/bin/env bash

arch="$(uname -m)"
busybox_version="1.35.0"
repo_root=$(git rev-parse --show-toplevel)
target_dir="${repo_root}/.invokex/runtime"
tmp_working_dir=$(mktemp -d)
trap "sudo rm -rf ${tmp_working_dir}" EXIT
pushd $tmp_working_dir

# Ensure no old rootfs is present anymore.
rm -f $target_dir/rootfs.ext4
mkdir -p $target_dir

rootfsName="rootfs"
mkdir $rootfsName
pushd $rootfsName

# Userspace initialisation.
cp "${repo_root}/config/init" ./init
chmod +x ./init

# Standard directory structure
mkdir bin sbin etc dev proc sys dev tmp

# Busybox will be used as a shell.
wget -O bin/busybox https://busybox.net/downloads/binaries/${busybox_version}-${arch}-linux-musl/busybox
chmod +x bin/busybox

ln -s /bin/busybox bin/sh
ln -s /bin/busybox bin/ls
ln -s /bin/busybox bin/mount

popd # out of rootfs

# Convert to ext4.
sudo chown -R root:root ${rootfsName}
truncate -s 64MB "${target_dir}/${rootfsName}.ext4"
sudo mkfs.ext4 -d $rootfsName -F "${target_dir}/${rootfsName}.ext4"

popd # back to the original location where the script was called from
