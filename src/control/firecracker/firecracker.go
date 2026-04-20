package firecracker

import (
	"fmt"
	"os/exec"
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

	vm struct {
		cmd *exec.Cmd
	}

	FirecrackerConfig struct {
		FirecrackerPath  string
		KernelImagePath  string
		KernelRootFsPath string
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

func (m *FirecrackerManager) newVmId(runtime Runtime) VmId {
	id := fmt.Sprintf("%s_%s", runtime, uuid.NewString())

	if _, exists := m.activeVms[id]; !exists {
		return id
	}

	// Edge case: duplicate UUID was generated so keep trying until an unused UUID is found.
	return m.newVmId(runtime)

}

func (m *FirecrackerManager) InstantiateVm(runtime Runtime) (VmId, error) {
	vm := vm{
		cmd: exec.Command(
			m.config.FirecrackerPath,
			fmt.Sprintf(`--api-sock ""`),    // TODO: create socket
			fmt.Sprintf(`--config-file ""`), // TODO: create config file
			"--enable-pci",
		),
	}

	if err := vm.cmd.Start(); err != nil {
		return "", err
	}

	id := m.newVmId(runtime)
	m.activeVms[id] = vm

	fmt.Printf("instantiated new %s vm: id %s\n", runtime, id)
	return id, nil
}

func (m *FirecrackerManager) KillVm(id VmId) error {
	if vm, exists := m.activeVms[id]; exists {
		return vm.cmd.Process.Kill()
	}

	return fmt.Errorf(`microvm not found: %s`, id)
}

func NewRuntime(name string, optVersion ...string) Runtime {
	if len(optVersion) > 0 && len(strings.TrimSpace(optVersion[0])) > 0 {
		return Runtime(fmt.Sprintf("%s_%v", name, optVersion[0]))
	}

	return Runtime(name)
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
