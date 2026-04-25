package main

import (
	"context"
	"fmt"

	"github.com/tntmeijs/invokex/src/configuration"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
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

	createFilesystemQueue := config.MessageBroker.MustGetQueueDetails("create_filesystem")
	consumer, err := connection.NewConsumer(mainCtx, createFilesystemQueue.Name, func() { exit <- true })
	if err != nil {
		panic(fmt.Sprintf("could not create a consumer: %s", err.Error()))
	}

	defer consumer.Stop(mainCtx)
	consumer.Listen(mainCtx, func(ctx context.Context, m rabbitmq.Message) rabbitmq.MessageOutcome {
		fmt.Println("processor received a message from the filesystem queue")
		return rabbitmq.MessageOutcomeAccept
	})

	fmt.Println("processor running")

	<-exit
	close(exit)

	fmt.Println("processor exited")
}
