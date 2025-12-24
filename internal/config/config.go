package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port               int      `mapstructure:"port"`
		CorsAllowedOrigins []string `mapstructure:"cors_allowed_origins"`
		CorsAllowedMethods []string `mapstructure:"cors_allowed_methods"`
		CorsAllowedHeaders []string `mapstructure:"cors_allowed_headers"`
	} `mapstructure:"server"`

	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Name     string `mapstructure:"name"`
	} `mapstructure:"database"`

	JWT struct {
		Secret          string `mapstructure:"secret"`
		ExpirationHours int    `mapstructure:"expiration_hours"`
		Issuer          string `mapstructure:"issuer"`
	} `mapstructure:"jwt"`

	G struct {
		Enabled bool `mapstructure:"enabled"`
		DB      struct {
			Host     string `mapstructure:"host"`
			Port     int    `mapstructure:"port"`
			User     string `mapstructure:"user"`
			Password string `mapstructure:"password"`
			Name     string `mapstructure:"name"`
		} `mapstructure:"db"`
	} `mapstructure:"g"`
}

func Load() *Config {
	// Load .env file if exists (ignore error in production)
	godotenv.Load()

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile("configs/config.yaml")

	// Auto bind environment variables
	v.AutomaticEnv()

	// Set sensible defaults (binary works without config file)
	v.SetDefault("server.port", 8080)
	v.SetDefault("jwt.expiration_hours", 24)
	v.SetDefault("jwt.issuer", "cold-backend")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.name", "cold_db")

	// Config file is optional
	if err := v.ReadInConfig(); err != nil {
		log.Printf("[Config] No config file found, using defaults")
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("config unmarshal error: %v", err)
	}

	// Override JWT secret from environment if not set
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "${JWT_SECRET}" {
		cfg.JWT.Secret = os.Getenv("JWT_SECRET")
		if cfg.JWT.Secret == "" {
			log.Fatal("JWT_SECRET environment variable is required")
		}
	}

	// Override G DB password from environment if enabled
	if cfg.G.Enabled {
		if cfg.G.DB.Password == "" || cfg.G.DB.Password == "${G_DB_PASSWORD}" {
			cfg.G.DB.Password = os.Getenv("G_DB_PASSWORD")
		}
	}

	return &cfg
}
