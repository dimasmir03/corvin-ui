package rabbitmq

import (
	"log"

	"github.com/streadway/amqp"
)

var Conn *amqp.Connection
var Channel *amqp.Channel

func Init(url string) {
	var err error
	Conn, err = amqp.Dial(url)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
	}

	Channel, err = Conn.Channel()
	if err != nil {
		log.Fatal("Failed to open channel:", err)
	}
}

func Publish(queue string, body []byte) error {
	_, err := Channel.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	return Channel.Publish("", queue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
