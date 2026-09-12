package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type AppConfig struct {
	Env  string // "dev" или "prod"
	Port string
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Issuer     string
}

type Config struct {
	App   AppConfig
	DB    DBConfig
	Redis RedisConfig
	NATS  NATSConfig
	JWT   JWTConfig
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type NATSConfig struct {
	Host string
	Port string
}

// Load собирает Config из переменных окружения.
// Никогда не паникует — для отсутствующих значений использует fallback.
func Load() *Config {
	return &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "dev"),
			Port: getEnv("APP_PORT", "8080"),
		},
		DB: DBConfig{
			Host:            getEnv("POSTGRES_HOST", "localhost"),
			Port:            getEnv("POSTGRES_PORT", "5432"),
			User:            getEnv("POSTGRES_USER", "postgres"),
			Password:        getEnv("POSTGRES_PASSWORD", "postgres"),
			Name:            getEnv("POSTGRES_DB", "vyborok"),
			SSLMode:         getEnv("POSTGRES_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("POSTGRES_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("POSTGRES_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		NATS: NATSConfig{
			Host: getEnv("NATS_HOST", "localhost"),
			Port: getEnv("NATS_PORT", "4222"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "dev-secret-change-me-in-prod"),
			AccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
			Issuer:     getEnv("JWT_ISSUER", "vyborok"),
		},
	}
}

// DSN возвращает строку подключения для lib/pq.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// Addr возвращает адрес NATS-сервера в формате host:port.
func (c NATSConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// Addr возвращает адрес Redis-сервера в формате host:port.
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// --- Вспомогательные функции ---

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
