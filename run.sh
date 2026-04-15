#!/usr/bin/env bash
set -e

# Always run from the repository's root directory.
repo_root=$(git rev-parse --show-toplevel)
pushd ${repo_root} > /dev/null

### ################### ###
### CUSTOM LINUX KERNEL ###
### ################### ###

# Build the custom Linux Kernel if it does not exist yet.
if [ ! -f "${repo_root}/.invokex/runtime/kernel.bin" ]; then
    chmod +x "${repo_root}/scripts/build_linux_kernel.sh"
    "${repo_root}/scripts/build_linux_kernel.sh"
else
    echo "Custom Linux kernel already built- if you would like to build it from scratch, manually invoke the ./scripts/build_linux_kernel.sh script."
fi

# Create a rootfs so the kernel can boot.
if [ ! -f "${repo_root}/.invokex/runtime/rootfs.ext4" ]; then
    chmod +x "${repo_root}/scripts/build_linux_rootfs.sh"
    "${repo_root}/scripts/build_linux_rootfs.sh"
else
    echo "Custom Linux rootfs already built- if you would like to build it from scratch, manually invoke the ./scripts/build_linux_rootfs.sh script."
fi

### ########### ###
### FIRECRACKER ###
### ########### ###

# Fresh firecracker installation if one does not exist yet.
if [ ! -d "${repo_root}/.invokex/firecracker" ]; then
    echo "Downloading latest Firecracker release..."
    chmod +x "${repo_root}/scripts/download_firecracker.sh"
    "${repo_root}/scripts/download_firecracker.sh"
else
    echo "Firecracker already installed - if you would like to install it from scratch, remove the ./.invokex/firecracker directory first."
fi

# Start firecracker.
chmod +x "${repo_root}/scripts/start_firecracker.sh"
"${repo_root}/scripts/start_firecracker.sh"

popd > /dev/null
