package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	for _, key := range []string{
		"PORT", "GEO_DB_PATH", "GEO_DB_REFRESH_INTERVAL",
		"REDIS_LOOKUP_KEY_PREFIX", "REDIS_LOOKUP_CACHE_TTL",
	} {
		t.Setenv(key, "")
	}
	cfg := Load()
	cases := []struct{ name, got, want string }{
		{"Port", cfg.Port, "8080"},
		{"GeoDBPath", cfg.GeoDBPath, "/tmp/geolite2-country.mmdb"},
		{"GeoDBRefreshInterval", cfg.GeoDBRefreshInterval, "24h"},
		{"RedisLookupKeyPrefix", cfg.RedisLookupKeyPrefix, "gogeoip:lookup:"},
		{"RedisLookupCacheTTL", cfg.RedisLookupCacheTTL, "24h"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("MAXMIND_ACCOUNT_ID", "acc123")
	t.Setenv("MAXMIND_LICENSE_KEY", "key456")
	t.Setenv("GEO_DB_PATH", "/data/geo.mmdb")
	t.Setenv("GEO_DB_REFRESH_INTERVAL", "12h")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_LOOKUP_KEY_PREFIX", "myapp:geo:")
	t.Setenv("REDIS_LOOKUP_CACHE_TTL", "1h")
	cfg := Load()
	cases := []struct{ name, got, want string }{
		{"Port", cfg.Port, "9090"},
		{"MaxmindAccountID", cfg.MaxmindAccountID, "acc123"},
		{"MaxmindLicenseKey", cfg.MaxmindLicenseKey, "key456"},
		{"GeoDBPath", cfg.GeoDBPath, "/data/geo.mmdb"},
		{"GeoDBRefreshInterval", cfg.GeoDBRefreshInterval, "12h"},
		{"RedisAddr", cfg.RedisAddr, "redis:6379"},
		{"RedisPassword", cfg.RedisPassword, "secret"},
		{"RedisLookupKeyPrefix", cfg.RedisLookupKeyPrefix, "myapp:geo:"},
		{"RedisLookupCacheTTL", cfg.RedisLookupCacheTTL, "1h"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestLoad_RedisAddrWhitespaceTrimmed(t *testing.T) {
	t.Setenv("REDIS_ADDR", "  localhost:6379  ")
	cfg := Load()
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr: got %q, want %q", cfg.RedisAddr, "localhost:6379")
	}
}

func TestGetEnv_ReturnsEnvVar(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "envvalue")
	if got := getEnv("TEST_GETENV_KEY", "fallback"); got != "envvalue" {
		t.Errorf("got %q, want %q", got, "envvalue")
	}
}

func TestGetEnv_ReturnsFallbackWhenEmpty(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "")
	if got := getEnv("TEST_GETENV_KEY", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}
