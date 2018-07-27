package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
)

var (
	errMissKafkaBrokers    = "queue(kafka): miss kafka brokers"
	errInvaildKafkaBrokers = "queue(kafka): invalid kafka brokers"
	errKafkaNewQueue       = "queue(kafka): new kafka error"
	errKafkaQueueNotExists = "queue(kafka): queue not exists"
)

type KafkaHandler struct {

	queues  map[string]*KafkaQueue
}

func NewKafkaHandler() Handler {
	return &KafkaHandler{}
}

func (this *KafkaHandler) Initiate(ctx context.Context) error {
	this.queues = make(map[string]*KafkaQueue)
	return nil
}
func (this *KafkaHandler) NewQueue(ctx context.Context, name string, config map[string]interface{}) (QueueHandler, error) {
	queue := &KafkaQueue{}
	configBrokers := config["QUEUE_KAFKA_BROKERS"]
	if configBrokers != nil {
		brokers, ok := configBrokers.(string)
		if !ok {
			return nil, errors.New(errInvaildKafkaBrokers)
		} else {
			queue.brokers = strings.Split(brokers, ",")
			queue.config = config
		}
	} else {
		return nil, errors.New(errMissKafkaBrokers)
	}
	return queue, nil
}

func (this *KafkaHandler) GetQueue(name string) (QueueHandler, error) {
	if this.queues == nil {
		return nil, errors.New(fmt.Sprintf(errKafkaQueueNotExists, name))
	}
	queueHandler, ok := this.queues[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf(errKafkaQueueNotExists, name))
	}
	return queueHandler, nil
}

type KafkaQueue struct {
	config map[string]interface{}
	brokers []string
}

func (this *KafkaQueue) GetConfig() map[string]interface{} {
	return this.config
}

// 创建生产者
func (this *KafkaQueue) NewProducer(ctx context.Context, topic string, message []byte, errHandles ...func(context.Context, string, string, error)) (*Producer, error) {
	config := sarama.NewConfig()
	config.ChannelBufferSize = 2000
	p, err := sarama.NewAsyncProducer(this.brokers, config)
	if err != nil {
		return nil, err
	}

	producer := &Producer{
		topic:    topic,
		message:  message,
		producer: p,
	}
	if len(errHandles) > 0 {
		go producer.startProducerErrorsHandle(ctx, errHandles[0])
	} else {
		go producer.startProducerErrorsHandle(ctx, handleProducerError)
	}

	return producer, nil
}

// 创建消费者
func (this *KafkaQueue) NewConsumer(ctx context.Context, groupName string, topic string) (*Consumer, error) {
	group, err := cluster.NewConsumer(this.brokers, groupName, []string{topic}, nil)
	if err != nil {
		return nil, err
	}

	ret := &Consumer{
		topic:     topic,
		groupName: groupName,
		offset:    make(map[int32]int64),
		group:     group,
	}
	return ret, nil
}

type Consumer struct {
	topic     string
	offset    map[int32]int64
	groupName string
	group     *cluster.Consumer
}

func (c *Consumer) Messages() <-chan *sarama.ConsumerMessage {
	return c.group.Messages()
}

func (c *Consumer) MarkOffset(message *sarama.ConsumerMessage) {
	c.offset[message.Partition] = message.Offset
	c.group.MarkOffset(message, c.groupName)
}

// 关闭消费者
func (c *Consumer) Close() {
	c.group.Close()
}

type Producer struct {
	topic    string
	message  []byte
	producer sarama.AsyncProducer
}

// 生产
func (p *Producer) Produce() {
	producerMessage := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(p.message),
	}
	p.producer.Input() <- producerMessage
}

// 启动写入错误处理
func (p *Producer) startProducerErrorsHandle(ctx context.Context, handle func(context.Context, string, string, error)) {
	for {
		producerErr := <-p.producer.Errors()
		message, _ := producerErr.Msg.Value.Encode()
		handle(ctx, producerErr.Msg.Topic, string(message), producerErr.Err)
	}
}

// 写入错误处理
func handleProducerError(ctx context.Context, topic string, message string, err error) {
	log.Printf("Kafka写失败 topic: %s message: %s error: %v", topic, message, err)
}
