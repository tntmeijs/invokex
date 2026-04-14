#!/usr/bin/env bash
set -e

# TODO: make this script configurable via arguments once multiple runtimes are supported
repo_root=$(git rev-parse --show-toplevel)
firecracker_dir="${repo_root}/.invokex/firecracker"
api_socket="${firecracker_dir}/firecracker.socket"
vmconfig_path="${repo_root}/.invokex/runtimes/alpine/vmconfig.json"
vm_name="test_vm"
log_directory="${repo_root}/.invokex/logs"
log_file="${log_directory}/firecracker_${vm_name}.log"
log_level="Debug"
kernel_boot_args="console=ttyS0 reboot=k panic=1"
kernel_image_path="${repo_root}/.invokex/runtimes/alpine/vmlinux"
kernel_rootfs_path="${repo_root}/.invokex/runtimes/alpine/rootfs.ext4"
network_interface_id="net1"
host_dev_name="tap0"
mac_address="06:00:AC:10:00:02" # 192.168.0.2

mkdir -p $log_directory

ARCH=$(uname -m)
if [ ${ARCH} = "aarch64" ]; then
    kernel_boot_args="keep_bootcon ${kernel_boot_args}"
fi

# Instead of configuring the VM using Firecracker's API over UNIX socket, we will use a configuration file instead.
# Since this file might have to be dynamic, we will generate it programmatically.
json=$(cat <<-END
    {
        "boot-source": {
            "kernel_image_path": "${kernel_image_path}",
            "boot_args": "${kernel_boot_args}"
        },
        "drives": [
            {
                "drive_id": "rootfs",
                "path_on_host": "${kernel_rootfs_path}",
                "is_root_device": true,
                "is_read_only": true
            }
        ],
        "machine-config": {
            "vcpu_count": 2,
            "mem_size_mib": 1024,
            "smt": false,
            "track_dirty_pages": false,
            "huge_pages": "None"
        },
        "network-interfaces": [
            {
                "iface_id": "${network_interface_id}",
                "guest_mac": "${mac_address}",
                "host_dev_name": "${host_dev_name}"
            }
        ],
        "logger": {
            "log_path": "${log_file}",
            "level": "${log_level}",
            "show_level": true,
            "show_log_origin": true
        },
        "pmem": []
    }
END
)

echo $json | jq "." > $vmconfig_path

# Race condition: the configuration file might not have been written yet so wait for it.
while [ ! -f $vmconfig_path ]; do sleep 0.250s; done

echo "CONFIG FILE PATH USED TO START FIRECRACKER: $vmconfig_path"

trap "sudo rm -f ${api_socket}" EXIT
sudo "${firecracker_dir}/firecracker" --api-sock "${api_socket}" --config-file "${vmconfig_path}" --enable-pci
