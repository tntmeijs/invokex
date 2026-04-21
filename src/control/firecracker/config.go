package firecracker

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type (
	vmConfig struct {
		fileName          string             `json:"-"`
		BootSource        bootSource         `json:"boot-source"`
		Drives            []drive            `json:"drives"`
		MachineConfig     machineConfig      `json:"machine-config"`
		NetworkInterfaces []networkInterface `json:"network-interfaces"`
		Logger            logger             `json:"logger"`
	}

	bootSource struct {
		KernelImagePath string `json:"kernel_image_path"`
		BootArgs        string `json:"boot_args"`
	}

	drive struct {
		Id           string `json:"drive_id"`
		PathOnHost   string `json:"path_on_host"`
		IsRootDevice bool   `json:"is_root_device"`
		IsReadOnly   bool   `json:"is_read_only"`
	}

	machineConfig struct {
		VcpuCount       int    `json:"vcpu_count"`
		MemorySize      int    `json:"mem_size_mib"`
		SMT             bool   `json:"smt"`
		TrackDirtyPages bool   `json:"track_dirty_pages"`
		HugePages       string `json:"huge_pages"`
	}

	networkInterface struct {
		Id              string `json:"iface_id"`
		GuestMacAddress string `json:"guest_mac"`
		HostDevName     string `json:"host_dev_name"`
	}

	logger struct {
		Path          string   `json:"log_path"`
		Level         LogLevel `json:"level"`
		ShowLevel     bool     `json:"show_level"`
		ShowLogOrigin bool     `json:"show_log_origin"`
	}
)

// CreateDefaultFirecrackerVmConfig creates a new FirecrackerVmConfig with sensible defaults.
func CreateDefaultFirecrackerVmConfig(
	id VmId,
	vmConfigDirectory, kernelImagePath, rootFsPath, logPath string,
	logLevel LogLevel,
) vmConfig {
	bootArgs := "console=ttyS0 reboot=k panic=1 init=./init"

	arch := os.Getenv("GOARCH")
	if strings.Compare(strings.ToLower(arch), "arm64") == 0 {
		bootArgs = "keep_bootcon " + bootArgs
	}

	return vmConfig{
		fileName: fmt.Sprintf("%s/%s_config.json", vmConfigDirectory, id),
		BootSource: bootSource{
			KernelImagePath: kernelImagePath,
			BootArgs:        "console=ttyS0 reboot=k panic=1 init=./init",
		},
		Drives: []drive{
			{
				Id:           "rootfs",
				PathOnHost:   rootFsPath,
				IsRootDevice: true,
				IsReadOnly:   false,
			},
		},
		MachineConfig: machineConfig{
			VcpuCount:       2,
			MemorySize:      1024,
			SMT:             false,
			TrackDirtyPages: false,
			HugePages:       "None",
		},
		NetworkInterfaces: []networkInterface{},
		Logger: logger{
			Path:          fmt.Sprintf("%s/%s.log", logPath, id),
			Level:         logLevel,
			ShowLevel:     true,
			ShowLogOrigin: true,
		},
	}
}

// WriteToDisk stores the FirecrackerVmConfig as a JSON file on disk.
func (c vmConfig) WriteToDisk() error {
	bytes, err := json.MarshalIndent(c, "", "  ") // pretty-print JSON for debugging purposes
	if err != nil {
		return fmt.Errorf("failed to marshal firecracker vm configuration: %w", err)
	}

	return os.WriteFile(c.fileName, bytes, 0644)
}

// The delete method removes the FirecrackerVmConfig JSON file from disk.
func (c vmConfig) delete() error {
	return os.Remove(c.fileName)
}
