package main

import (
	"github.com/tntmeijs/invokex/src/control/application"
	"github.com/tntmeijs/invokex/src/control/config"
	"github.com/tntmeijs/invokex/src/control/firecracker"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
)

var dependencyProviderFuncs = []any{
	provideGlobalConfig,
	provideFirecrackerConfig,
	provideFirecrackerManager,
	provideRabbitMqInstance,
	provideFileUploadProcessor,
}

func provideGlobalConfig() (config.Config, error) {
	cfg := config.MustLoadFromArgs()
	return cfg, cfg.CreateDirectories()
}

func provideFirecrackerConfig(c config.Config) firecracker.FirecrackerConfig {
	return firecracker.FirecrackerConfig{
		FirecrackerPath:     c.Firecracker.Instance.Path,
		KernelImagePath:     c.Firecracker.Kernel.Path,
		KernelRootFsPath:    c.Firecracker.RootFilesystem.Path,
		LogDirectory:        c.Firecracker.Directories.FirecrackerLogs,
		VmConfigDirectory:   c.Firecracker.Directories.VmConfigurations,
		ApiSocketsDirectory: c.Firecracker.Directories.ApiSockets,
		VmLogsDirectory:     c.Firecracker.Directories.VmLogs,
	}
}

func provideFirecrackerManager(c firecracker.FirecrackerConfig) firecracker.FirecrackerManager {
	return firecracker.NewManager(c)
}

func provideRabbitMqInstance(c config.Config) rabbitmq.Instance {
	return rabbitmq.NewInstance(c.MessageBroker.Username, c.MessageBroker.Password, c.MessageBroker.Host)
}

func provideFileUploadProcessor() application.FileUploadProcessor {
	return application.NewFileUploadProcessor()
}
