package config

import (
	"os"
	"strings"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port                 string
	MaxmindAccountID     string
	MaxmindLicenseKey    string
	GeoDBPath            string
	GeoDBRefreshInterval string
	RedisAddr            string
	RedisPassword        string
	RedisLookupKeyPrefix string
	RedisLookupCacheTTL  string
}

// Load reads configuration from environment variables.
func Load() Config {
	return Config{
		Port:                 getEnv("PORT", "8080"),
		MaxmindAccountID:     os.Getenv("MAXMIND_ACCOUNT_ID"),
		MaxmindLicenseKey:    os.Getenv("MAXMIND_LICENSE_KEY"),
		GeoDBPath:            getEnv("GEO_DB_PATH", "/tmp/geolite2-country.mmdb"),
		GeoDBRefreshInterval: getEnv("GEO_DB_REFRESH_INTERVAL", "24h"),
		RedisAddr:            strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		RedisPassword:        os.Getenv("REDIS_PASSWORD"),
		RedisLookupKeyPrefix: getEnv("REDIS_LOOKUP_KEY_PREFIX", "gogeoip:lookup:"),
		RedisLookupCacheTTL:  getEnv("REDIS_LOOKUP_CACHE_TTL", "24h"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
