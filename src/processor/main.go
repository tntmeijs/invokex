package main

import (
	"context"
	"fmt"

	"github.com/tntmeijs/invokex/src/configuration"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
)

const (
	applicationExchangeName   string = "invokex.user.application"
	createFilesystemQueueName string = "user.application.archive.filesystem"
	unpackArchiveQueueName    string = "user.application.archive.unpack"
)

// TODO: wrap application with a signal listener so we can clean up when we receive SIGTERM.
func main() {
	exit := make(chan bool)
	config := configuration.MustLoadFromArgs()
	mainCtx := context.Background()

	instance := rabbitmq.NewInstance("processor", config.MessageBroker.Username, config.MessageBroker.Password, config.MessageBroker.Host)
	defer instance.Close(mainCtx)

	connection, err := instance.Connect(mainCtx, config.MessageBroker.Queues, config.MessageBroker.Exchanges)
	if err != nil {
		panic(fmt.Sprintf("could not establish a connection with rabbitmq: %s", err.Error()))
	}

	// TODO: build a better configuration abstraction for RabbitMQ - this is not user friendly...
	userApplicationExchange := config.MessageBroker.MustGetExchangeDetails(applicationExchangeName)
	bindingKey := userApplicationExchange.Bindings[createFilesystemQueueName].BindingKey

	filesystemPublisher, err := connection.NewExchangePublisher(mainCtx, applicationExchangeName, bindingKey)
	if err != nil {
		panic(fmt.Sprintf("could not create application ext4 filesystem publisher: %s", err.Error()))
	}
	defer filesystemPublisher.Stop(mainCtx)

	unpackArchiveConsumer, err := connection.NewConsumer(mainCtx, unpackArchiveQueueName)
	if err != nil {
		panic(fmt.Sprintf("could not create unpack archive consumer: %v", err.Error()))
	}

	archiveUnpacker := NewArchiveUnpacker(config.Application.Upload.Output, filesystemPublisher)
	closeUnpackArchiveConsumer := unpackArchiveConsumer.Listen(mainCtx, archiveUnpacker.onEvent)
	defer closeUnpackArchiveConsumer()

	createFilesystemConsumer, err := connection.NewConsumer(mainCtx, createFilesystemQueueName)
	if err != nil {
		panic(fmt.Sprintf("could not create filesystem consumer: %v", err.Error()))
	}

	filesystemBuilder := NewFilesystemBuilder(config.Application.Filesystems.Directory)
	closeCreateFilesystemConsumer := createFilesystemConsumer.Listen(mainCtx, filesystemBuilder.onEvent)
	defer closeCreateFilesystemConsumer()

	fmt.Println("processor running")

	<-exit
	close(exit)

	fmt.Println("processor exited")
}
