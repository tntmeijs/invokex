#!/usr/bin/env bash
set -e

repo_root=$(git rev-parse --show-toplevel)
firecracker_dir="${repo_root}/.invokex/firecracker"
api_socket="${firecracker_dir}/firecracker.socket"

sudo curl -X PUT --unix-socket "${api_socket}" \
    --data "{
        \"action_type\": \"InstanceStart\"
    }" \
    "http://localhost/actions"
