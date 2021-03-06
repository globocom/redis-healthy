// REDIS-HEALTHY DOCUMENTATION
//
// Package main provides a program that retrieves metrics, periodically, from Redis (or sentinel)
// (such as latency, connected_clients, instantaneous_ops_per_sec and others) and then send it to Logstash.
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/redis.v5"
)

func main() {
	log.Println("starting")
	config, err := getConfig()

	if err != nil {
		log.Fatal(err.Error())
	}

	ticker := time.NewTicker(time.Duration(config.pingFrequency) * time.Second)

	for _ = range ticker.C {
		_, err := ping(config)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	log.Println("finished")
}

// getConfig gets all the config from enviroment variables
func getConfig() (configuration, error) {
	var err error
	config := configuration{}

	config.redisHost, err = getMandatoryParameter("REDIS_HOST")
	if err != nil {
		return config, err
	}

	config.logstashHost, err = getMandatoryParameter("LOGSTASH_HOST")
	if err != nil {
		return config, err
	}

	config.logstashPort, err = getMandatoryParameter("LOGSTASH_PORT")
	if err != nil {
		return config, err
	}

	config.logstashProtocol = getParameter("LOGSTASH_PROTOCOL")
	if config.logstashProtocol == "" {
		config.logstashProtocol = "udp"
	}

	config.redisMasterName = getParameter("REDIS_MASTER_NAME")
	config.redisPwd = getParameter("REDIS_PWD")
	config.redisSentinel = getParameter("REDIS_SENTINEL")

	if config.redisSentinel != "" && config.redisMasterName == "" {
		return config, errors.New("When you're using sentinel you must provide the env REDIS_MASTER_NAME")
	}

	config.redisLatencyThreshold = getParameter("REDIS_LATENCY_THRESHOLD")

	// a list of fields one want to measure separated by , ex: "client_longest_output_list,connected_clients"
	redisMetricsToWatch := getParameter("REDIS_METRICS_TO_WATCH")

	if redisMetricsToWatch == "" {
		redisMetricsToWatch = "client_longest_output_list,connected_clients,blocked_clients,rejected_connections,instantaneous_input_kbps,instantaneous_output_kbps,instantaneous_ops_per_sec,keyspace_hits,keyspace_misses,mem_fragmentation_ratio,sync_full,sync_partial_ok,sync_partial_err"
	}

	config.pingFrequency, _ = strconv.Atoi(getParameter("PING_FREQUENCY"))

	if config.pingFrequency == 0 {
		config.pingFrequency = 10
	}

	config.redisListMetricsToWatch = strings.Split(redisMetricsToWatch, ",")

	config.project, err = getMandatoryParameter("PROJECT")
	if err != nil {
		return config, err
	}

	return config, nil
}

// ping gets and sends metrics periodically
func ping(config configuration) (map[string]interface{}, error) {
	log.Println("connecting to redis")
	redisClient := createRedis(config)
	defer redisClient.Close()

	info, err := getInfo(redisClient)
	if err != nil {
		return nil, err
	}

	metrics := parse(info, config.redisListMetricsToWatch)

	latency, err := getLatency(redisClient, config)
	if err != nil {
		return nil, err
	}

	if latency > -1 {
		metrics["latency"] = latency
	}

	var sender sender
	sender = logstash{
		Host:      config.logstashHost,
		Port:      config.logstashPort,
		Protocol:  config.logstashProtocol,
		Namespace: config.project,
	}

	sender.send(metrics)

	return metrics, nil
}

// getInfo gets info from redis instance (from command "INFO")
func getInfo(redisClient *redis.Client) (string, error) {
	infoResponse, err := redisClient.Info().Result()
	if err != nil {
		return "", err
	}

	return infoResponse, nil
}

// configuration holds the data needed for the application
type configuration struct {
	redisHost               string
	logstashHost            string
	logstashPort            string
	logstashProtocol        string
	redisMasterName         string
	redisPwd                string
	redisSentinel           string
	redisLatencyThreshold   string
	redisListMetricsToWatch []string
	pingFrequency           int
	project                 string
}

// getMandatoryParameter gets enviroment variable value
// if it's empty it'll raise an error
func getMandatoryParameter(key string) (string, error) {
	value := getParameter(key)
	if value == "" {
		return "", errors.New("You must provide the env " + key)
	}
	return value, nil
}

// getParameter gets enviroment variable value
func getParameter(key string) string {
	return os.Getenv(key)
}

// createRedis creates a redis instance
func createRedis(config configuration) *redis.Client {
	if config.redisSentinel != "" {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    config.redisMasterName,
			Password:      config.redisPwd,
			SentinelAddrs: strings.Split(config.redisHost, ","),
		})
	}
	return redis.NewClient(&redis.Options{
		Addr:     config.redisHost,
		Password: config.redisPwd,
		DB:       0,
	})
}

// parse parses the info response from redis to a map containing each metric
func parse(infoResult string, metricsToWatch []string) map[string]interface{} {
	regexFieldsPattern := strings.Join(metricsToWatch, "|")
	regularProperty := regexp.MustCompile("(" + regexFieldsPattern + "):([[:alnum:]]+)")
	allRedisFields := regularProperty.FindAllStringSubmatch(infoResult, -1)

	metrics := make(map[string]interface{})

	for _, element := range allRedisFields {
		key := element[1]
		value, _ := strconv.Atoi(element[2])
		metrics[key] = value
	}

	return metrics
}

// getLatency fetches the latest latency according with the threshold
// it'll reset the latency (0) when it has passed PING_FREQUENCY time
func getLatency(redisClient *redis.Client, config configuration) (int64, error) {
	if config.redisLatencyThreshold != "" {
		redisClient.ConfigSet("latency-monitor-threshold", config.redisLatencyThreshold)
		log.Println("starting ping")
		latency, err := measureLatency(redisClient, config.pingFrequency)
		log.Println("ending ping")
		return latency, err
	}
	return -1, nil
}

// measureLatency deals with redis command latency to get the latest latency
func measureLatency(client *redis.Client, pingFrequency int) (int64, error) {
	cmd := redis.NewSliceCmd("latency", "latest")
	if err := client.Process(cmd); err != nil {
		return int64(0), err
	}
	var latest int64
	rawValue := cmd.Val()

	thereIsLatency := len(rawValue) > 0 && len(rawValue[0].([]interface{})) > 3

	if thereIsLatency {
		response := rawValue[0].([]interface{})
		latestCommandEpoch := time.Unix(response[1].(int64), 0)
		now := time.Now()
		diff := now.Sub(latestCommandEpoch)

		hasPassedTimeWithoutNewLatency := diff.Seconds() > float64(pingFrequency)

		if hasPassedTimeWithoutNewLatency {
			latest = 0
		} else {
			latest = response[2].(int64)
		}
	}

	return latest, nil
}

// sender is a type that sends metrics
type sender interface {
	send(data map[string]interface{}) (string, error)
}

// logstash is a sub-type of sender that sends metrics
type logstash struct {
	Host      string
	Port      string
	Protocol  string
	Namespace string
}

// send sends metrics to logstash in form of json
func (l logstash) send(data map[string]interface{}) (string, error) {
	// it creates a default client, ex: "project-redis"
	data["client"] = l.Namespace + "-redis"

	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	conn, err := net.DialTimeout(l.Protocol, l.Host+":"+l.Port, time.Duration(1)*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	log.Printf("sending metrics %s", b)

	n, err := conn.Write(b)
	if err != nil {
		return "", err
	}
	log.Println("the metrics were sent with " + strconv.Itoa(n) + " bytes")
	return "success", nil
}
