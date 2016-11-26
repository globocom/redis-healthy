package main

import (
	"os"
	"testing"

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
