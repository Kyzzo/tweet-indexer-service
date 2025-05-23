package kafka

import (
	"log"
	"net"
	"strconv"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/stream"
	"github.com/jeetdotso/tweet-indexer-service/util"
	"github.com/segmentio/kafka-go"
)

func CreateTopics(cfg *config.Config, topicNames []string) {

	conn, err := kafka.Dial("tcp", util.JoinStrings(cfg.Stream.Kafka.Brokers, ","))
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		panic(err.Error())
	}
	var controllerConn *kafka.Conn
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		panic(err.Error())
	}
	defer controllerConn.Close()

	topics := Topics(cfg.Stream)
	var cfgmap = map[string]kafka.TopicConfig{} // map of topics to create
	for _, tn := range topicNames {
		topic := stream.StreamTweetPrefixTopic + "." + tn
		if util.StringArrayContains(topics, topic) == false {
			cfgmap[topic] = kafka.TopicConfig{
				Topic:             topic,
				NumPartitions:     4,
				ReplicationFactor: -1,
			}
		}
	}

	var topicsCfg []kafka.TopicConfig
	for _, v := range cfgmap {
		topicsCfg = append(topicsCfg, v)
	}

	err = controllerConn.CreateTopics(topicsCfg...)
	if err != nil {
		// panic(err.Error())
		log.Println("Error creating topics: ", err.Error())
	}
}

func Topics(streamCfg config.Stream) []string {
	conn, err := kafka.Dial("tcp", util.JoinStrings(streamCfg.Kafka.Brokers, ","))
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		panic(err.Error())
	}

	m := map[string]struct{}{}

	for _, p := range partitions {
		m[p.Topic] = struct{}{}
	}

	var topics []string
	for k := range m {
		topics = append(topics, k)
	}

	return topics
}
