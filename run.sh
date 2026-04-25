#!/usr/bin/env bash
set -e

runtimes=("golang")

if ! [[ ${runtimes[*]} =~ $1 ]]
then
    echo "Please use a supported runtime:"
    echo "  - golang"
    exit 1
fi

# Kill all processes whose parent PID is the current PID of the script sending SIGINT.
# https://stackoverflow.com/a/35660327
trap terminate SIGINT
terminate(){
    pkill -SIGINT -P $$
    exit
}

runtime="$1"
architecture="$(uname -m)"

# Always run from the repository's root directory.
repo_root=$(git rev-parse --show-toplevel)
pushd "${repo_root}" > /dev/null

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
if [ ! -f "${repo_root}/.invokex/runtime/rootfs_${runtime}_${architecture}.ext4" ]; then
    chmod +x "${repo_root}/scripts/build_linux_rootfs.sh"
    "${repo_root}/scripts/build_linux_rootfs.sh" "${runtime}"
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

### ################ ###
### PROCESSOR WORKER ###
### ################ ###

# This is a worker of the control plane. The reason it is its own application is that this improves resiliency.
# Additionally, the worker creates filesystems, which requires elevated permissions.
# Isolating elevated privileges to just this application, results in increased system safety (principle of least privilege).

go build -C "${repo_root}/src/processor"
chmod +x "${repo_root}/src/processor"

### ############# ###
### CONTROL PLANE ###
### ############# ###

go build -C "${repo_root}/src/control"
chmod +x "${repo_root}/src/control"

# Launch all applications.
"${repo_root}/src/processor/processor" &
"${repo_root}/src/control/control" &
wait

popd > /dev/null
