package redis

import (
	"log"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/martinohansen/ynabber"
)

// Client returns a redis.NewClient using the environment config
func Client() *redis.Client {
	addr := ynabber.ConfigLookup("REDIS_ADDRESE", "")
	password := ynabber.ConfigLookup("REDIS_PASSWORD", "")
	string_db := ynabber.ConfigLookup("REDIS_PASSWORD", "0")
	db, err := strconv.Atoi(string_db)
	if err != nil {
		log.Fatalf("invalid db ID, conversion of %s to int failed: %s", string_db, err)
	}

	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}
