package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

type (
	MessageOutcome int

	ConnectionOption  func(context.Context, *rabbitmqamqp.AmqpManagement) error
	OnMessageReceived func(Message) MessageOutcome

	// Instance provides a way to interact with RabbitMQ instances.
	Instance struct {
		Environment *rabbitmqamqp.Environment
	}

	// Connection represents a single RabbitMQ connection.
	Connection struct {
		Raw *rabbitmqamqp.AmqpConnection
	}

	// Consumer represents a consumer that processes messages from a queue.
	Consumer struct {
		Raw  *rabbitmqamqp.Consumer
		done chan bool
	}

	Publisher struct {
		Raw *rabbitmqamqp.Publisher
	}

	// Message is a consumable message from a queue.
	Message struct {
		Data []byte
	}
)

const (
	MessageOutcomeAccept MessageOutcome = iota
	MessageOutcomeDiscard
	MessageOutcomeRequeue
)

// NewInstance interfaces with RabbitMQ.
func NewInstance(username, password, host string) Instance {
	e := rabbitmqamqp.NewEnvironment(fmt.Sprintf("amqp://%s:%s@%s", username, password, host), nil)
	return Instance{Environment: e}
}

// Connect opens a connection to RabbitMQ.
func (i *Instance) Connect(ctx context.Context, options ...ConnectionOption) (Connection, error) {
	c, err := i.Environment.NewConnection(ctx)
	if err != nil {
		return Connection{}, fmt.Errorf("failed to create rabbitmq connection: %w", err)
	}

	connection := Connection{Raw: c}
	management := connection.Raw.Management()

	for _, option := range options {
		if err = option(ctx, management); err != nil {
			connection.Raw.Close(ctx)
			return Connection{}, err
		}
	}

	if err = management.Close(ctx); err != nil {
		connection.Raw.Close(ctx)
		return Connection{}, fmt.Errorf("failed to apply connection options: %w", err)
	}

	return connection, nil
}

// Close cleans up the resources allocated by Instance.
func (i *Instance) Close(ctx context.Context) error {
	return i.Environment.CloseConnections(ctx)
}

// NewConsumer creates a new Consumer for a Connection.
func (c *Connection) NewConsumer(ctx context.Context, queue string) (Consumer, error) {
	consumer, err := c.Raw.NewConsumer(ctx, queue, nil)
	if err != nil {
		return Consumer{}, fmt.Errorf("failed to create new consumer: %w", err)
	}

	return Consumer{Raw: consumer, done: make(chan bool)}, nil
}

// NewExchangePublisher creates a new Publisher that targets a specific exchange (and optionally a routing key).
func (c *Connection) NewExchangePublisher(ctx context.Context, exchange string, routingKey ...string) (Publisher, error) {
	addr := rabbitmqamqp.ExchangeAddress{Exchange: exchange}
	if len(routingKey) > 0 {
		addr.Key = routingKey[0]
	}

	p, err := c.Raw.NewPublisher(ctx, &addr, nil)
	return Publisher{Raw: p}, err
}

// NewQueuePublisher creates a new Publisher that targets a specific queue.
func (c *Connection) NewQueuePublisher(ctx context.Context, queue string) (Publisher, error) {
	addr := rabbitmqamqp.QueueAddress{Queue: queue}
	p, err := c.Raw.NewPublisher(ctx, &addr, nil)
	return Publisher{Raw: p}, err
}

// Listen starts the receive loop that listens to the queue the Consumer is configured for.
func (c *Consumer) Listen(ctx context.Context, onMessage OnMessageReceived) {
	// TODO: add logging on failures in the consumer Goroutine
	// TODO: add better error handling in Goroutine
	go func(consumerCtx context.Context) {
		for {
			select {
			case <-c.done:
				c.Raw.Close(consumerCtx)
				return
			default:
				deliveryCtx, err := c.Raw.Receive(consumerCtx)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					} else {
						// Unhandled error - send it back to the queue.
						// TODO: see if we can include the error here somehow
						deliveryCtx.Discard(consumerCtx, nil)
					}
				}

				switch onMessage(Message{Data: deliveryCtx.Message().GetData()}) {
				case MessageOutcomeAccept:
					deliveryCtx.Accept(consumerCtx)
				case MessageOutcomeDiscard:
					deliveryCtx.Discard(consumerCtx, nil)
				case MessageOutcomeRequeue:
					deliveryCtx.Requeue(consumerCtx)
				}
			}
		}
	}(ctx)
}

// Stop quits the message processing Goroutine.
func (c *Consumer) Stop() {
	c.done <- true
	close(c.done)
}

// SendJson marshals the message object into JSON data and publishes the message.
func (p Publisher) SendJson(ctx context.Context, message any) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to send json message: %w", err)
	}

	result, err := p.Raw.Publish(ctx, rabbitmqamqp.NewMessage(bytes))
	if err != nil {
		return err
	}

	switch result.Outcome.(type) {
	case *rabbitmqamqp.StateAccepted:
		return nil
	case *rabbitmqamqp.StateReleased:
		return fmt.Errorf("failed to send message because it could not be routed")
	case *rabbitmqamqp.StateRejected:
		stateType := result.Outcome.(*rabbitmqamqp.StateRejected)
		if stateType.Error != nil {
			return fmt.Errorf("failed to send message because it was rejected: %w", stateType.Error)
		}

		return fmt.Errorf("failed to send message because it was rejected")
	}

	return nil
}

// Stop disconnects the publisher.
func (p Publisher) Stop(ctx context.Context) error {
	return p.Raw.Close(ctx)
}

// AsJson is a utility method that attempts to unmarshal the message as JSON.
func (m Message) AsJson(out any) error {
	return json.Unmarshal(m.Data, out)
}

// WithClassicQueue ensures a classic queue with the specified name is present when the connection is established.
func WithClassicQueue(name string) ConnectionOption {
	return func(ctx context.Context, m *rabbitmqamqp.AmqpManagement) error {
		_, err := m.DeclareQueue(ctx, &rabbitmqamqp.ClassicQueueSpecification{Name: name})
		return err
	}
}

// WithTopicExchange ensures a topic exchange with the specified name is present when the connection is established.
func WithTopicExchange(name string) ConnectionOption {
	return func(ctx context.Context, m *rabbitmqamqp.AmqpManagement) error {
		_, err := m.DeclareExchange(ctx, &rabbitmqamqp.TopicExchangeSpecification{Name: name})
		return err
	}
}

// WithExchangeToQueueBinding binds an exchange (source) to a queue (destination) based on the binding key (key) specified.
func WithExchangeToQueueBinding(source, destination, key string) ConnectionOption {
	return func(ctx context.Context, m *rabbitmqamqp.AmqpManagement) error {
		_, err := m.Bind(ctx, &rabbitmqamqp.ExchangeToQueueBindingSpecification{SourceExchange: source, DestinationQueue: destination, BindingKey: key})
		return err
	}
}
