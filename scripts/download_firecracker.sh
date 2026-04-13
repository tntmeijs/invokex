#!/usr/bin/env bash
# Original adapted from: https://github.com/firecracker-microvm/firecracker/blob/main/docs/getting-started.md#getting-a-firecracker-binary
set -e

# Remove the entire folder because it might be a partially broken installation.
repo_root=$(git rev-parse --show-toplevel)
target_dir="${repo_root}/.invokex/firecracker"
rm -rf ${target_dir}
mkdir -p ${target_dir}

# Work from a temporary directory to avoid polluting the user's working direcotyr.
tmp_working_dir=$(mktemp -d)
trap "rm -rf ${tmp_working_dir}" EXIT
pushd ${tmp_working_dir} > /dev/null

ARCH="$(uname -m)"
release_url="https://github.com/firecracker-microvm/firecracker/releases"
latest=$(basename $(curl -fsSLI -o /dev/null -w  %{url_effective} ${release_url}/latest))

# Download
curl -L ${release_url}/download/${latest}/firecracker-${latest}-${ARCH}.tgz | tar -xz

# Grab all useful binaries
mv release-${latest}-${ARCH}/firecracker-${latest}-${ARCH} ${target_dir}/firecracker
mv release-${latest}-${ARCH}/jailer-${latest}-${ARCH} ${target_dir}/jailer

# Clean up after ourselves
popd > /dev/null
