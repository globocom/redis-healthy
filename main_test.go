package main

import (
	"os"
	"testing"

	redis "gopkg.in/redis.v5"

	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
	redisInfo := `# Server
redis_version:3.2.0
tcp_port:6379
uptime_in_seconds:21536

# Clients
connected_clients:1

# Memory
used_memory:7751008
used_memory_human:7.39M

# Stats
total_connections_received:6111
latest_fork_usec:995
  `
	metrics := parse(redisInfo, []string{"used_memory", "connected_clients"})

	assert.Equal(t, metrics["used_memory"].(int), 7751008)
	assert.Equal(t, metrics["connected_clients"].(int), 1)
}

func TestGetConfig(t *testing.T) {
	oldValue := getParameter("PING_FREQUENCY")
	os.Setenv("PING_FREQUENCY", "42")

	config, err := getConfig()

	assert.Nil(t, err)
	assert.Equal(t, config.pingFrequency, 42)

	os.Setenv("PING_FREQUENCY", oldValue)
}

func TestDefaultConfig(t *testing.T) {
	config, err := getConfig()

	assert.Nil(t, err)
	assert.Equal(t, config.logstashProtocol, "udp")
	assert.Contains(t, config.redisListMetricsToWatch, "connected_clients")
	assert.Equal(t, config.pingFrequency, 10)
}

func TestGetConfigWithoutMandatory(t *testing.T) {
	oldValue := getParameter("LOGSTASH_HOST")
	os.Setenv("LOGSTASH_HOST", "")
	_, err := getConfig()

	assert.NotNil(t, err)

	os.Setenv("LOGSTASH_HOST", oldValue)
}

func TestWhenYouUseSentinelWithMasterName(t *testing.T) {
	oldValueRedisSentinel := getParameter("REDIS_SENTINEL")
	oldValueRedisMasterName := getParameter("REDIS_MASTER_NAME")
	os.Setenv("REDIS_SENTINEL", "Y")
	os.Setenv("REDIS_MASTER_NAME", "master")
	_, err := getConfig()

	assert.Nil(t, err)

	os.Setenv("REDIS_SENTINEL", oldValueRedisSentinel)
	os.Setenv("REDIS_MASTER_NAME", oldValueRedisMasterName)
}

func TestWhenYouUseSentinelWithoutMasterName(t *testing.T) {
	oldValueRedisSentinel := getParameter("REDIS_SENTINEL")
	os.Setenv("REDIS_SENTINEL", "Y")
	_, err := getConfig()

	assert.NotNil(t, err)

	os.Setenv("REDIS_SENTINEL", oldValueRedisSentinel)
}

func TestGetInfo(t *testing.T) {
	config, _ := getConfig()
	redisClient := createRedis(config)
	defer redisClient.Close()

	info, err := getInfo(redisClient)

	assert.Nil(t, err)
	assert.NotEqual(t, info, "")
}

func TestPing(t *testing.T) {
	config, err := getConfig()
	assert.Nil(t, err)

	metrics, err := ping(config)
	assert.Nil(t, err)

	assert.NotEqual(t, len(metrics), 0)
	assert.Equal(t, metrics["connected_clients"].(int), 1)
}

func TestGetLatency(t *testing.T) {
	oldValueRedisLatencyThreshold := getParameter("REDIS_LATENCY_THRESHOLD")
	// setting a new latency threshold (ms)
	os.Setenv("REDIS_LATENCY_THRESHOLD", "50")

	config, err := getConfig()
	assert.Nil(t, err)

	metrics, err := ping(config)
	assert.Nil(t, err)

	assert.Equal(t, metrics["latency"].(int64), int64(0))

	// simulate a a high latency command `debug sleep .1` (sleep for 100 ms)
	redisClient := createRedis(config)
	defer redisClient.Close()
	cmd := redis.NewStringCmd("debug", "sleep", "0.1")
	err = redisClient.Process(cmd)
	assert.Nil(t, err)

	// check the latency
	metrics, err = ping(config)
	assert.Nil(t, err)

	assert.Equal(t, metrics["latency"].(int64) >= 100, true)

	// teardown
	os.Setenv("REDIS_LATENCY_THRESHOLD", oldValueRedisLatencyThreshold)
}
