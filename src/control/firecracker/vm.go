package firecracker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type (
	onVmProcessExit = func(VmId, int)

	vm struct {
		id              VmId
		cmd             *exec.Cmd
		onVmProcessExit onVmProcessExit
		socket          apiSocket
		config          vmConfig
	}
)

func newVm(id VmId, firecrackerBinaryPath, vmConfigDirectory, apiSocketDirectory, kernelImagePath, rootFsPath, firecrackerLogDirectory string, onExit onVmProcessExit) (vm, error) {
	vmCfg := CreateDefaultFirecrackerVmConfig(id, vmConfigDirectory, kernelImagePath, rootFsPath, firecrackerLogDirectory, LogLevelDebug)
	if err := vmCfg.WriteToDisk(); err != nil {
		return vm{}, fmt.Errorf("failed to write vm %s configuration to disk: %w", id, err)
	}

	// Ensure Firecracker has a log destination.
	_, err := os.OpenFile(vmCfg.Logger.Path, os.O_CREATE, 0666)
	if err != nil {
		return vm{}, fmt.Errorf("failed to create firecracker log file: %w", err)
	}

	socket := newApiSocket(id, apiSocketDirectory)

	cmd := exec.Command(
		firecrackerBinaryPath,
		"--api-sock", socket.path,
		"--config-file", vmCfg.fileName,
		"--enable-pci",
	)

	return vm{
		id:              id,
		cmd:             cmd,
		onVmProcessExit: onExit,
		socket:          socket,
		config:          vmCfg,
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

	vm.onVmProcessExit(vm.id, exitCode)
}
