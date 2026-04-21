package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type (
	Config struct {
		Firecracker Firecracker `json:"firecracker"`
	}

	Firecracker struct {
		Instance       BinaryFile  `json:"instance"`
		Kernel         BinaryFile  `json:"kernel"`
		RootFilesystem BinaryFile  `json:"rootfs"`
		Directories    Directories `json:"directories"`
	}

	BinaryFile struct {
		Path string `json:"path"`
	}

	Directories struct {
		FirecrackerLogs  string `json:"firecrackerlogs"`
		VmConfigurations string `json:"vmconfigs"`
		ApiSockets       string `json:"apisockets"`
	}
)

// Load attempts to load the configuration file from disk.
func Load(path string) (Config, error) {
	var c Config

	bytes, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("unable to read configuration file: %w", err)
	}

	fmt.Print("loaded control plane configuration from file\n\n")
	fmt.Printf("%s\n\n", string(bytes))

	if err = json.Unmarshal(bytes, &c); err != nil {
		return c, fmt.Errorf("unable to unmarshal configuration file: %w", err)
	}

	return c, nil
}

// MustLoadFromArgs attempts to load a configuration file from disk using the path provided by the --config=<path> flag.
func MustLoadFromArgs() Config {
	var configPath string
	flag.StringVar(&configPath, "config", "./invokex.json", "path to the invokex configuration file")
	flag.Parse()

	config, err := Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("could not load configuration: %v", err))
	}

	return config
}
