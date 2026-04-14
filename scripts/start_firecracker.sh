#!/usr/bin/env bash
set -e

repo_root=$(git rev-parse --show-toplevel)
firecracker_dir="${repo_root}/.invokex/firecracker"
api_socket="${firecracker_dir}/firecracker.socket"

trap "sudo rm -f ${api_socket}" EXIT
sudo "${firecracker_dir}/firecracker" --api-sock "${api_socket}" --enable-pci
