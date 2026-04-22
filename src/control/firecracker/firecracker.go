package firecracker

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type (
	// The VmId is a unique identifier for a microvm.
	VmId = string

	// LogLevel defines Firecracker's logger's verbosity.
	LogLevel string

	// Runtime is the application type that runs in the microvm.
	Runtime string

	VmConfig interface {
		LogLevel() LogLevel
		Runtime() Runtime
	}

	FirecrackerConfig struct {
		FirecrackerPath     string
		KernelImagePath     string
		KernelRootFsPath    string
		LogDirectory        string
		VmConfigDirectory   string
		ApiSocketsDirectory string
		VmLogsDirectory     string
	}

	FirecrackerManager struct {
		config    FirecrackerConfig
		vmConfigs map[Runtime]VmConfig
		activeVms map[VmId]vm
	}

	baseVmConfig struct {
		logLevel LogLevel
	}

	golangVmConfig struct {
		baseVmConfig
	}

	nodeVmConfig struct {
		baseVmConfig
		version int
	}
)

const (
	LogLevelError   LogLevel = "Error"
	LogLevelWarning LogLevel = "Warning"
	LogLevelInfo    LogLevel = "Info"
	LogLevelDebug   LogLevel = "Debug"
	LogLevelTrace   LogLevel = "Trace"
	LogLevelOff     LogLevel = "Off"
)

func NewManager(config FirecrackerConfig) FirecrackerManager {
	return FirecrackerManager{
		config:    config,
		vmConfigs: map[Runtime]VmConfig{},
		activeVms: map[VmId]vm{},
	}
}

func (m *FirecrackerManager) RegisterVmConfig(config VmConfig) error {
	if _, exists := m.vmConfigs[config.Runtime()]; exists {
		return fmt.Errorf(`a vm config with id "%s" already exists`, config.Runtime())
	}

	m.vmConfigs[config.Runtime()] = config
	return nil
}

func (m *FirecrackerManager) InstantiateVm(runtime Runtime) (VmId, error) {
	vm, err := m.newVm(runtime, func(vmId VmId, exitCode int) {
		delete(m.activeVms, vmId)

		if len(m.activeVms) == 0 {
			fmt.Println("no active virtual machines")
		} else {
			fmt.Println("active virtual machines:")

			for id := range m.activeVms {
				fmt.Printf("  - %s\n", id)
			}
		}
	})

	if err != nil {
		return "", fmt.Errorf("failed to instantiate new vm: %w", err)
	}

	vm.start()

	fmt.Printf("instantiated new %s vm: id %s\n", runtime, vm.id)
	m.activeVms[vm.id] = vm
	return vm.id, nil
}

func (m *FirecrackerManager) KillVm(id VmId) error {
	if vm, exists := m.activeVms[id]; exists {
		return vm.cmd.Process.Kill()
	}

	return fmt.Errorf(`microvm not found: %s`, id)
}

func NewRuntime(name string, optVersion ...string) Runtime {
	if len(optVersion) > 0 && len(strings.TrimSpace(optVersion[0])) > 0 {
		return Runtime(strings.ToLower(fmt.Sprintf("%s_%v", name, optVersion[0])))
	}

	return Runtime(strings.ToLower(name))
}

func NewGolangConfig(logLevel LogLevel) VmConfig {
	return golangVmConfig{
		baseVmConfig: baseVmConfig{
			logLevel: logLevel,
		},
	}
}

func (c golangVmConfig) LogLevel() LogLevel {
	return c.logLevel
}

func (c golangVmConfig) Runtime() Runtime {
	return NewRuntime("golang")
}

func NewNodeConfig(nodeVersion int, logLevel LogLevel) VmConfig {
	return nodeVmConfig{
		baseVmConfig: baseVmConfig{
			logLevel: logLevel,
		},
		version: nodeVersion,
	}
}

func (c nodeVmConfig) LogLevel() LogLevel {
	return c.logLevel
}

func (c nodeVmConfig) Runtime() Runtime {
	return NewRuntime("node", strconv.Itoa(c.version))
}

// The newVm method creates a new virtual machine but does not start it yet.
func (m *FirecrackerManager) newVm(runtime Runtime, onExit onVmProcessExit) (vm, error) {
	var existingIds []VmId
	for key := range m.activeVms {
		existingIds = append(existingIds, key)
	}

	newVmId := NewVmId(runtime, existingIds...)

	newVmConfigurationInfo := vmCreateInfo{
		firecrackerBinaryPath:    m.config.FirecrackerPath,
		vmConfigurationDirectory: m.config.VmConfigDirectory,
		apiSocketDirectory:       m.config.ApiSocketsDirectory,
		kernelImagePath:          m.config.KernelImagePath,
		rootFsPath:               m.config.KernelRootFsPath,
		firecrackerLogDirectory:  m.config.LogDirectory,
		vmLogsDirectory:          m.config.VmLogsDirectory,
	}

	return newVm(newVmId, newVmConfigurationInfo, onExit)
}

// NewVmId returns a new VmId based on the runtime.
// Optionally, existing ids can be passed to ensure uniqueness.
func NewVmId(runtime Runtime, existingIds ...VmId) VmId {
	id := fmt.Sprintf("%s_%s", runtime, uuid.NewString())

	if !slices.Contains(existingIds, id) {
		return id
	}

	// Edge case: duplicate UUID was generated so keep trying until an unused UUID is found.
	existingIds = append(existingIds, id)
	return NewVmId(runtime, existingIds...)
}
