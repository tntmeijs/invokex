package firecracker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

	onVmProcessExit = func(VmId, int)

	VmConfig interface {
		LogLevel() LogLevel
		Runtime() Runtime
	}

	vm struct {
		id     VmId
		cmd    *exec.Cmd
		onExit onVmProcessExit
	}

	FirecrackerConfig struct {
		FirecrackerPath   string
		KernelImagePath   string
		KernelRootFsPath  string
		LogDirectory      string
		VmConfigDirectory string
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
				fmt.Printf("  - %s", id)
			}
		}
	})

	if err != nil {
		return "", fmt.Errorf("failed to instantiate new vm: %w", err)
	}

	vm.Start()

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

// The newVm method creates a new virtual machine but does not start it yet.
func (m *FirecrackerManager) newVm(runtime Runtime, onExit onVmProcessExit) (vm, error) {
	var existingIds []VmId
	for key, _ := range m.activeVms {
		existingIds = append(existingIds, key)
	}

	// TODO: move this elsewhere and generate new values instead of duplicates - probably store these in the vm structure
	cfg := CreateDefaultFirecrackerVmConfig(
		runtime,
		m.config.KernelImagePath,
		m.config.KernelRootFsPath,
		"net1",
		"06:00:AC:10:00:02",
		"tap0",
		m.config.LogDirectory,
		LogLevelDebug,
	)

	newVmId := NewVmId(runtime, existingIds...)

	fileName, err := cfg.WriteToDisk(newVmId, m.config.VmConfigDirectory)
	if err != nil {
		return vm{}, fmt.Errorf("failed to write vm %s configuration to disk: %w", newVmId, err)
	}

	return vm{
		id: newVmId,
		cmd: exec.Command(
			m.config.FirecrackerPath,
			fmt.Sprintf(`--api-sock ""`), // TODO: create socket
			fmt.Sprintf(`--config-file "%s"`, fileName),
			"--enable-pci",
		),
		onExit: func(id VmId, exitCode int) {
			// Inject ourselves here - once the VM exists, clean up its configuration file.
			os.Remove(fileName)
			fmt.Printf("cleanup: %s\n", fileName)

			// End of injection.
			onExit(id, exitCode)
		},
	}, nil
}

func (vm *vm) Start() {
	go func() {
		err := vm.cmd.Run()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			fmt.Printf(
				"vm %s (PID %d) exited with exit code %d and stderr: \"%s\"\n",
				vm.id,
				exitErr.Pid(),
				exitErr.ExitCode(),
				string(exitErr.Stderr),
			)

			vm.onExit(vm.id, exitErr.ExitCode())
		} else {
			vm.onExit(vm.id, 0)
		}
	}()
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
