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
	config := configuration()
	pingFrequency, _ := strconv.Atoi(config["pingFrequency"].(string))
	ticker := time.NewTicker(time.Duration(pingFrequency) * time.Second)

	for _ = range ticker.C {
		ping()
	}

	log.Println("finished")
}

func ping() {
	log.Println("starting ping")

	config := configuration()
	frequency, _ := strconv.Atoi(config["pingFrequency"].(string))
	redisClient := createRedis(config)
	defer redisClient.Close()

	log.Println("connected to redis")

	infoResponse, err := redisClient.Info().Result()
	checkError(err)

	allRedisFields := parse(infoResponse, config["redisListMetricsToWatch"].([]string))

	metrics := make(map[string]interface{})

	for _, element := range allRedisFields {
		key := element[1]
		value, _ := strconv.Atoi(element[2])
		metrics[key] = value
	}

	latestLatency, err := fetchLatestLatency(redisClient, config["redisLatencyThreshold"].(string), frequency)
	checkError(err)

	if latestLatency > -1 {
		metrics["latest-latency"] = latestLatency
	}

	var sender sender
	sender = logstash{Host: config["logstashHost"].(string), Port: config["logstashPort"].(string), Protocol: "udp", Namespace: config["project"].(string)}

	sender.Send(metrics)
	log.Println("all the metrics were sent")
	log.Println("ending ping")
}

func configuration() map[string]interface{} {
	config := make(map[string]interface{})

	redisHost, err := fetchMandatoryParameter("REDIS_HOST")
	checkError(err)
	config["redisHost"] = redisHost

	logstashHost, err := fetchMandatoryParameter("LOGSTASH_HOST")
	checkError(err)
	config["logstashHost"] = logstashHost

	logstashPort, err := fetchMandatoryParameter("LOGSTASH_PORT")
	checkError(err)
	config["logstashPort"] = logstashPort

	config["redisMasterName"] = fetchParameter("REDIS_MASTER_NAME")
	config["redisPwd"] = fetchParameter("REDIS_PWD")
	config["redisSentinel"] = fetchParameter("REDIS_SENTINEL")
	config["redisLatencyThreshold"] = fetchParameter("REDIS_LATENCY_THRESHOLD")

	// a list of fields one want to measure separated by , ex: "client_longest_output_list,connected_clients"
	config["redisMetricsToWatch"] = fetchParameter("REDIS_METRICS_TO_WATCH")

	if config["redisMetricsToWatch"] == "" {
		config["redisMetricsToWatch"] = "client_longest_output_list,connected_clients,blocked_clients,rejected_connections,instantaneous_input_kbps,instantaneous_output_kbps,instantaneous_ops_per_sec,keyspace_hits,keyspace_misses,mem_fragmentation_ratio,sync_full,sync_partial_ok,sync_partial_err"
	}

	config["pingFrequency"] = fetchParameter("PING_FREQUENCY")

	if config["pingFrequency"] == "" {
		config["pingFrequency"] = "10"
	}

	config["redisListMetricsToWatch"] = strings.Split(config["redisMetricsToWatch"].(string), ",")

	project, err := fetchMandatoryParameter("PROJECT")
	checkError(err)
	config["project"] = project

	return config
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}
}

func fetchMandatoryParameter(key string) (string, error) {
	value := fetchParameter(key)
	if value == "" {
		return "", errors.New("You must provide the env " + key)
	}
	return value, nil
}

func fetchParameter(key string) string {
	return os.Getenv(key)
}

func createRedis(config map[string]interface{}) *redis.Client {
	if config["redisSentinel"] != "" {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    config["redisMasterName"].(string),
			Password:      config["redisPwd"].(string),
			SentinelAddrs: strings.Split(config["redisHost"].(string), ","),
		})
	}
	return redis.NewClient(&redis.Options{
		Addr:     config["redisHost"].(string),
		Password: config["redisPwd"].(string),
		DB:       0,
	})
}

// each sub array contains the wanted fields and their values
// ex: ["client_longest_output_list:2" "client_longest_output_list" 2"]
func parse(result string, metricsToWatch []string) [][]string {
	regexFieldsPattern := strings.Join(metricsToWatch, "|")
	regularProperty := regexp.MustCompile("(" + regexFieldsPattern + "):([[:alnum:]]+)")
	return regularProperty.FindAllStringSubmatch(result, -1)
}

// it fetches the latest latency according with the threshold
// it'll reset the latency (0) when it has passed PING_FREQUENCY time
func fetchLatestLatency(redisClient *redis.Client, redisLatencyThreshold string, frequency int) (int64, error) {
	if redisLatencyThreshold != "" {
		redisClient.ConfigSet("latency-monitor-threshold", redisLatencyThreshold)
		return measureLatency(redisClient, frequency)

	}
	return -1, nil
}

func measureLatency(client *redis.Client, frequency int) (int64, error) {
	cmd := redis.NewSliceCmd("latency", "latest")
	if err := client.Process(cmd); err != nil {
		return 0, err
	}
	var latest int64 = -1
	rawValue := cmd.Val()

	if len(rawValue) > 0 && len(rawValue[0].([]interface{})) > 3 {
		response := rawValue[0].([]interface{})
		latestCommandEpoch := time.Unix(response[1].(int64), 0)
		now := time.Now()
		diff := now.Sub(latestCommandEpoch)

		if diff.Seconds() > float64(frequency) {
			latest = 0
		} else {
			latest = response[2].(int64)
		}
	}

	return latest, nil
}

type sender interface {
	Send(data map[string]interface{}) (string, error)
}

type logstash struct {
	Host      string
	Port      string
	Protocol  string
	Namespace string
}

func (l logstash) Send(data map[string]interface{}) (string, error) {
	// it creates a default client, ex: "project-redis"
	data["client"] = l.Namespace + "-redis"
	log.Println("sending metrics")

	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	conn, err := net.DialTimeout(l.Protocol, l.Host+":"+l.Port, time.Duration(1)*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	log.Println("sending metrics")

	n, err := conn.Write(b)
	if err != nil {
		return "", err
	}
	log.Println("the metrics were sent with " + strconv.Itoa(n) + " bytes")
	return "success", nil
}
