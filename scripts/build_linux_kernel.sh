#!/usr/bin/env bash
# Reference: https://github.com/firecracker-microvm/firecracker/blob/main/docs/rootfs-and-kernel-setup.md
set -e

repo_root=$(git rev-parse --show-toplevel)
target_dir="${repo_root}/.invokex/runtime"
pushd "${repo_root}/submodules/linux"

# Clear out the old kernel.
rm -f "${target_dir}/kernel.bin"
mkdir -p $target_dir

arch=$(uname -m)

case $arch in
  x86_64)  export ARCH=x86 ;;
  aarch64) export ARCH=arm64 ;;
  *) echo "Unsupported architecture"; exit 1 ;;
esac

# Ensure we use the InvokeX configuration.
rm -f "./.config" "./.config.old"
cp -f "${repo_root}/config/linux_kernel_${arch}.config" "./.config"

make CC=$(which gcc-11) olddefconfig

if [ "$arch" = "x86_64" ]; then
    make CC=$(which gcc-14) -j$(nproc) vmlinux
    cp ./vmlinux "${target_dir}/kernel.bin"
elif [ "$arch" = "aarch64" ]; then
    make CC=$(which gcc-14) -j$(nproc) Image
    cp ./arch/arm64/boot/Image "${target_dir}/kernel.bin"
fi

popd
