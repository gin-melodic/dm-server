package integration

import (
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestConfig holds all configurations for integration and stability tests
type TestConfig struct {
	BaseURL        string
	WSURL          string
	JWTSecret      string
	Concurrency    int
	Duration       time.Duration
	JitterMax      time.Duration
	Latency        time.Duration
	DisconnectProb float64
}

// LoadConfig loads test configuration from environment variables with sensible defaults
func LoadConfig() TestConfig {
	cfg := TestConfig{
		BaseURL:        getEnv("TEST_HTTP_URL", "http://localhost:8000/api"),
		WSURL:          getEnv("TEST_WS_URL", "ws://localhost:8000/api/v1/chat/ws"),
		JWTSecret:      getEnv("TEST_JWT_SECRET", "f1805dd9597735822a397c0af13b45bccf00e9ec24fb7356d26abbbed51fef5d"),
		Concurrency:    getEnvInt("TEST_CONCURRENCY", 10),
		Duration:       time.Duration(getEnvInt("TEST_DURATION_SEC", 10)) * time.Second,
		JitterMax:      time.Duration(getEnvInt("TEST_JITTER_MAX_MS", 200)) * time.Millisecond,
		Latency:        time.Duration(getEnvInt("TEST_LATENCY_MS", 50)) * time.Millisecond,
		DisconnectProb: getEnvFloat("TEST_DISCONNECT_PROB", 0.1),
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

// GenerateTestToken signs a valid JWT token for test requests
func GenerateTestToken(userID uint64, openID string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"userId": userID,
		"openid": openID,
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
