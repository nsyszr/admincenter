package stats

import (
	"fmt"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

type Collector struct {
	redisClient *redis.Client
}

func NewCollector(redisClient *redis.Client) *Collector {
	return &Collector{
		redisClient: redisClient,
	}
}

func (c *Collector) UpdateSessionIOStats(realm, remoteAddr string, rxMsgs, txMsgs, rxBytes, txBytes int64) {
	key := fmt.Sprintf("stats:%s:io", remoteAddr)

	keys, err := c.redisClient.Keys(key).Result()
	if err != nil {
		log.Error("Failed to update stats: ", err)
		return
	}

	// Stats entry for realm doesnt exists. Add a new one.
	if len(keys) == 0 {
		fields := make(map[string]interface{})
		fields["realm"] = ""
		fields["rx_msgs"] = 0
		fields["tx_msgs"] = 0
		fields["rx_bytes"] = 0
		fields["tx_bytes"] = 0

		_, err := c.redisClient.HMSet(key, fields).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
			return
		}
	}

	// Update realm
	if realm != "" {
		_, err := c.redisClient.HSet(key, "realm", realm).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	// Update stats
	if rxMsgs != 0 {
		_, err := c.redisClient.HIncrBy(key, "rx_msgs", rxMsgs).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	if txMsgs != 0 {
		_, err := c.redisClient.HIncrBy(key, "tx_msgs", txMsgs).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	if rxBytes != 0 {
		_, err := c.redisClient.HIncrBy(key, "rx_bytes", rxBytes).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}

	if txBytes != 0 {
		_, err := c.redisClient.HIncrBy(key, "tx_bytes", txBytes).Result()
		if err != nil {
			log.Error("Failed to update stats: ", err)
		}
	}
}
