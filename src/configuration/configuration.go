package configuration

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

type (
	Configuration struct {
		MessageBroker MessageBroker `json:"messagebroker"`
		Application   Application   `json:"application"`
		Firecracker   Firecracker   `json:"firecracker"`
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
		VmLogs           string `json:"vmlogs"`
	}

	Application struct {
		Upload Upload `json:"upload"`
	}

	Upload struct {
		Directory string `json:"directory"`
		Output    string `json:"output"`
	}

	MessageBroker struct {
		Host      string                     `json:"host"`
		Username  string                     `json:"username"`
		Password  string                     `json:"password"`
		Queues    map[string]QueueDetails    `json:"queues"`
		Exchanges map[string]ExchangeDetails `json:"exchanges"`
	}

	QueueDetails struct {
		Name string `json:"name"`
	}

	ExchangeDetails struct {
		Name     string            `json:"name"`
		Bindings []ExchangeBinding `json:"bindings"`
	}

	ExchangeBinding struct {
		QueueId    string `json:"id"`
		BindingKey string `json:"key"`
	}
)

// Load attempts to load the configuration file from disk.
func Load(path string) (Configuration, error) {
	var c Configuration

	bytes, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("unable to read configuration file: %w", err)
	}

	if err = json.Unmarshal(bytes, &c); err != nil {
		return c, fmt.Errorf("unable to unmarshal configuration file: %w", err)
	}

	return c, nil
}

// MustLoadFromArgs attempts to load a configuration file from disk using the path provided by the --config=<path> flag.
func MustLoadFromArgs() Configuration {
	var configPath string
	flag.StringVar(&configPath, "invokex-config", "./invokex.json", "path to the invokex configuration file")
	flag.Parse()

	config, err := Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("could not load configuration: %v", err))
	}

	return config
}

// CreateDirectories ensures that the directories specified in the configuration are actually present on disk.
func (c Configuration) CreateDirectories() error {
	var errs error
	errors.Join(errs, os.MkdirAll(c.Application.Upload.Directory, 0744))
	errors.Join(errs, os.MkdirAll(c.Application.Upload.Output, 0744))
	errors.Join(errs, os.MkdirAll(c.Firecracker.Directories.ApiSockets, 0744))
	errors.Join(errs, os.MkdirAll(c.Firecracker.Directories.FirecrackerLogs, 0744))
	errors.Join(errs, os.MkdirAll(c.Firecracker.Directories.VmConfigurations, 0744))
	errors.Join(errs, os.MkdirAll(c.Firecracker.Directories.VmLogs, 0744))

	return errs
}

// MustGetQueueDetails fetches QueueDetails for the specified id.
// Panics if no entry for the given id has been configured.
func (m MessageBroker) MustGetQueueDetails(id string) QueueDetails {
	if details, exists := m.Queues[id]; exists {
		return details
	}

	panic(fmt.Sprintf(`no queue configured for id "%s"`, id))
}

// MustGetExchangeDetails fetches ExchangeDetails for the specified id.
// Panics if no entry for the given id has been configured.
func (m MessageBroker) MustGetExchangeDetails(id string) ExchangeDetails {
	if details, exists := m.Exchanges[id]; exists {
		return details
	}

	panic(fmt.Sprintf(`no exchange configured for id "%s"`, id))
}
