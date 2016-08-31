package app


import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

const (
	QUEUE_NAME="container-broker"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func InitQueue() error {

	conn, err := amqp.Dial(GetQueueConnectionString())
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	_, err = ch.QueueDeclare(
		QUEUE_NAME, // name
		true,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)

	failOnError(err, "Failed to declare a queue")
	return nil

}

func SendMessageToQueue(message []byte) error {
	conn, err := amqp.Dial(GetQueueConnectionString())
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.Publish(
		"",     // exchange
		QUEUE_NAME, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing {
			ContentType: "application/json",
			Body:        message,
		})
	failOnError(err, "Failed to publish a message")

	return nil
}