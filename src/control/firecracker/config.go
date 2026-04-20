package firecracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type (
	FirecrackerVmConfig struct {
		BootSource        BootSource         `json:"boot-source"`
		Drives            []Drive            `json:"drives"`
		MachineConfig     MachineConfig      `json:"machine-config"`
		NetworkInterfaces []NetworkInterface `json:"network-interfaces"`
		Logger            Logger             `json:"logger"`
	}

	BootSource struct {
		KernelImagePath string `json:"kernel_image_path"`
		BootArgs        string `json:"boot_args"`
	}

	Drive struct {
		Id           string `json:"drive_id"`
		PathOnHost   string `json:"path_on_host"`
		IsRootDevice bool   `json:"is_root_device"`
		IsReadOnly   bool   `json:"is_read_only"`
	}

	MachineConfig struct {
		VcpuCount       int    `json:"vcpu_count"`
		MemorySize      int    `json:"mem_size_mib"`
		SMT             bool   `json:"smt"`
		TrackDirtyPages bool   `json:"track_dirty_pages"`
		HugePages       string `json:"huge_pages"`
	}

	NetworkInterface struct {
		Id              string `json:"iface_id"`
		GuestMacAddress string `json:"guest_mac"`
		HostDevName     string `json:"host_dev_name"`
	}

	Logger struct {
		Path          string   `json:"log_path"`
		Level         LogLevel `json:"level"`
		ShowLevel     bool     `json:"show_level"`
		ShowLogOrigin bool     `json:"show_log_origin"`
	}
)

// CreateDefaultFirecrackerVmConfig creates a new FirecrackerVmConfig with sensible defaults.
func CreateDefaultFirecrackerVmConfig(
	vmId VmId,
	runtime Runtime,
	kernelImagePath, rootFsPath, logPath string,
	logLevel LogLevel,
) FirecrackerVmConfig {
	bootArgs := "console=ttyS0 reboot=k panic=1 init=./init"

	arch := os.Getenv("GOARCH")
	if strings.Compare(strings.ToLower(arch), "arm64") == 0 {
		bootArgs = "keep_bootcon " + bootArgs
	}

	return FirecrackerVmConfig{
		BootSource: BootSource{
			KernelImagePath: kernelImagePath,
			BootArgs:        "console=ttyS0 reboot=k panic=1 init=./init",
		},
		Drives: []Drive{
			{
				Id:           "rootfs",
				PathOnHost:   rootFsPath,
				IsRootDevice: true,
				IsReadOnly:   false,
			},
		},
		MachineConfig: MachineConfig{
			VcpuCount:       2,
			MemorySize:      1024,
			SMT:             false,
			TrackDirtyPages: false,
			HugePages:       "None",
		},
		NetworkInterfaces: []NetworkInterface{},
		Logger: Logger{
			Path:          fmt.Sprintf("%s/%s.log", logPath, vmId),
			Level:         logLevel,
			ShowLevel:     true,
			ShowLogOrigin: true,
		},
	}
}

// WriteToDisk stores the FirecrackerVmConfig as a JSON file on disk at the specified location.
func (c FirecrackerVmConfig) WriteToDisk(id VmId, directory string) (string, error) {
	bytes, err := json.MarshalIndent(c, "", "  ") // pretty-print JSON for debugging purposes
	if err != nil {
		return "", fmt.Errorf("failed to marshal firecracker vm configuration: %w", err)
	}

	// Create directory if not exists.
	_, err = os.Stat(directory)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to check if firecracker vm configuration directory exists: %w", err)
		}

		if err = os.MkdirAll(directory, 0755); err != nil {
			return "", fmt.Errorf("failed to create firecracker vm configuration directory: %w", err)
		}
	}

	fileName := fmt.Sprintf("%s/%s_config.json", directory, id)
	return fileName, os.WriteFile(fileName, bytes, 0644)
}
