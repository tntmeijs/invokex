#!/usr/bin/env bash
set -e

# Always run from the repository's root directory.
repo_root=$(git rev-parse --show-toplevel)
pushd ${repo_root} > /dev/null

# Fresh firecracker installation if one does not exist yet.
if [ ! -d "${repo_root}/.invokex/firecracker" ]; then
    echo "Downloading latest Firecracker release..."
    chmod +x "${repo_root}/scripts/download_firecracker.sh"
    "${repo_root}/scripts/download_firecracker.sh"
else
    echo "Firecracker already installed - if you would like to install it from scratch, remove the ./.invokex/firecracker directory first."
fi

# Build runtime if it does not exist yet.
if [ ! -d "${repo_root}/.invokex/runtimes/alpine" ]; then
    echo "Building Alpine Linux runtime..."
    chmod +x "${repo_root}/scripts/download_alpine_runtime.sh"
    "${repo_root}/scripts/download_alpine_runtime.sh"
else
    echo "Alpine Linux runtime already built- if you would like to build it from scratch, remove the ./.invokex/runtimes/alpine directory first."
fi

chmod +x "${repo_root}/scripts/start_firecracker.sh"
"${repo_root}/scripts/start_firecracker.sh"

popd > /dev/null
