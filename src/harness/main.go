// The harness module is responsible for user-space initialisation when the Linux kernel starts.
// Rather than starting a new shell, this binary will be invoked and it will act as the
// entrypoint of InvokeX. Meaning it will act as a harness for InvokeX applications.
//
// This is where we do general setup and inject InvokeX between the user-defined applications
// and the Linux kernel. Allowing us to do things such as montoring, logging, etc.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
)

const exitCmd = "exit"

func main() {
	mountKernelFs()
	entrypoint()
}

// The mountKernelFs function mounts the kernel-provided virtual filesystems other processes rely upon.
func mountKernelFs() error {
	// Process information.
	if err := syscall.Mount("none", "/proc", "proc", 0, ""); err != nil {
		return err
	}

	// Available hardware and kernel state.
	if err := syscall.Mount("none", "/sys", "sysfs", 0, ""); err != nil {
		return err
	}

	// Device interaction and basic I/O.
	if err := syscall.Mount("none", "/dev", "dev", 0, ""); err != nil {
		return err
	}

	return nil
}

func entrypoint() {
	fmt.Println("Welcome to InvokeX - this is where your application would be started.")

	buf := bufio.NewReader(os.Stdin)

	exit := false
	for !exit {
		fmt.Print("> ")
		input, err := buf.ReadBytes('\n')
		if err != nil {
			fmt.Println(err)
			continue
		}

		inputStr := strings.TrimSpace(strings.ToLower(string(input)))

		switch inputStr {
		case exitCmd:
			fmt.Println("Exiting VM...")
			exit = true
		default:
			fmt.Printf("Unsupported command \"%s\". Type \"%s\" to exit this VM.\n", inputStr, exitCmd)
		}
	}
}
