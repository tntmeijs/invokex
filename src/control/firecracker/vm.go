package firecracker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
)

type (
	onVmProcessExit = func(VmId, int)

	vm struct {
		id              VmId
		cmd             *exec.Cmd
		onVmProcessExit onVmProcessExit
		socket          apiSocket
		config          vmConfig
		stdout          os.File
		stderr          os.File
	}

	vmCreateInfo struct {
		firecrackerBinaryPath    string
		vmConfigurationDirectory string
		apiSocketDirectory       string
		kernelImagePath          string
		rootFsPath               string
		firecrackerLogDirectory  string
		vmLogsDirectory          string
	}
)

func newVm(id VmId, createInfo vmCreateInfo, onExit onVmProcessExit) (vm, error) {
	vmCfg := CreateDefaultFirecrackerVmConfig(
		id,
		createInfo.vmConfigurationDirectory,
		createInfo.kernelImagePath,
		createInfo.rootFsPath,
		createInfo.firecrackerLogDirectory,
		LogLevelDebug,
	)

	if err := vmCfg.WriteToDisk(); err != nil {
		return vm{}, fmt.Errorf("failed to write vm %s configuration to disk: %w", id, err)
	}

	// Ensure Firecracker has a log destination.
	_, err := os.OpenFile(vmCfg.Logger.Path, os.O_CREATE, 0666)
	if err != nil {
		vmCfg.delete()
		return vm{}, fmt.Errorf("failed to create firecracker log file: %w", err)
	}

	socket := newApiSocket(id, createInfo.apiSocketDirectory)

	cmd := exec.Command(
		createInfo.firecrackerBinaryPath,
		"--api-sock", socket.path,
		"--config-file", vmCfg.fileName,
		"--enable-pci",
	)

	stdoutDestination, err := os.Create(path.Join(createInfo.vmLogsDirectory, fmt.Sprintf("%s_stdout.log", id)))
	if err != nil {
		vmCfg.delete()
		return vm{}, fmt.Errorf("failed to create file for vm's stdout stream: %w", err)
	}

	stderrDestination, err := os.Create(path.Join(createInfo.vmLogsDirectory, fmt.Sprintf("%s_stderr.log", id)))
	if err != nil {
		vmCfg.delete()
		stdoutDestination.Close()

		return vm{}, fmt.Errorf("failed to create file for vm's stderr stream: %w", err)
	}

	cmd.Stdout = stdoutDestination
	cmd.Stderr = stderrDestination

	return vm{
		id:              id,
		cmd:             cmd,
		onVmProcessExit: onExit,
		socket:          socket,
		config:          vmCfg,
		stdout:          *stdoutDestination,
		stderr:          *stderrDestination,
	}, nil
}

func (vm *vm) start() {
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

			vm.onProcessEnd(exitErr.ExitCode())
		} else {
			vm.onProcessEnd(0)
		}
	}()
}

func (vm *vm) onProcessEnd(exitCode int) {
	vm.socket.close()
	vm.config.delete()
	vm.stdout.Close()
	vm.stderr.Close()

	vm.onVmProcessExit(vm.id, exitCode)
}
