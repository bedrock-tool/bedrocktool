package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type apiQueue struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
	err     chan *amqp.Error

	uri string

	connect_lock *sync.Mutex
}

func (c *apiQueue) Reconnect() {
	c.connect_lock.Lock()
	defer c.connect_lock.Unlock()
	if c.conn != nil {
		return
	}

	var err error
	for {
		c.conn, err = amqp.Dial(c.uri)
		if err == nil {
			break
		}

		logrus.Errorf("Error Connecting to RabbitMQ %s", err)
		logrus.Info("Trying to reconnect to RabbitMQ at %s", c.uri)
		time.Sleep(10 * time.Second)
	}

	c.err = make(chan *amqp.Error)
	c.conn.NotifyClose(c.err)

	c.channel, err = c.conn.Channel()
	if err != nil {
		panic(err)
	}

	c.queue, err = c.channel.QueueDeclare("player_skins", false, false, false, true, nil)
	if err != nil {
		panic(err)
	}
}

// Publish publishes a request to the amqp queue
func (c *apiQueue) Publish(ctx context.Context, data []byte) error {
	select { // non blocking channel - if there is no error will go to default where we do nothing
	case err := <-c.err:
		if err != nil {
			c.conn = nil
			c.Reconnect()
		}
	default:
	}

	p := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	}
	if err := c.channel.PublishWithContext(ctx, "", c.queue.Name, false, false, p); err != nil {
		return fmt.Errorf("error in Publishing: %s", err)
	}
	return nil
}
