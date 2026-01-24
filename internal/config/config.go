package config

import (
	"context"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	Razorpay struct {
		KeyID         string `mapstructure:"key_id"`
		KeySecret     string `mapstructure:"key_secret"`
		WebhookSecret string `mapstructure:"webhook_secret"`
	} `mapstructure:"razorpay"`

	BackupDir string // Local backup directory
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

	// Set BackupDir from env or default
	if dir := os.Getenv("BACKUP_DIR"); dir != "" {
		cfg.BackupDir = dir
	} else {
		cfg.BackupDir = "/mass-pool/backups"
	}

	// Override database settings from DB_* environment variables
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if n, err := strconv.Atoi(port); err == nil && n > 0 {
			cfg.Database.Port = n
		}
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.Database.User = user
	}
	if pass := os.Getenv("DB_PASSWORD"); pass != "" {
		cfg.Database.Password = pass
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		cfg.Database.Name = name
	}

	// Override JWT secret from environment if not set
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "${JWT_SECRET}" {
		cfg.JWT.Secret = os.Getenv("JWT_SECRET")
		if cfg.JWT.Secret == "" {
			// Try to fetch from R2 backup (disaster recovery)
			log.Printf("[Config] JWT_SECRET not set, fetching from R2 backup...")
			cfg.JWT.Secret = fetchJWTSecretFromR2()
			if cfg.JWT.Secret == "" {
				log.Fatal("JWT_SECRET not found in environment or R2 backup")
			}
			log.Printf("[Config] JWT secret loaded from R2 backup")
		}
	}

	// Override G DB password from environment if enabled
	if cfg.G.Enabled {
		if cfg.G.DB.Password == "" || cfg.G.DB.Password == "${G_DB_PASSWORD}" {
			cfg.G.DB.Password = os.Getenv("G_DB_PASSWORD")
		}
	}

	// Load Razorpay config from environment variables
	if keyID := os.Getenv("RAZORPAY_KEY_ID"); keyID != "" {
		cfg.Razorpay.KeyID = keyID
	}
	if keySecret := os.Getenv("RAZORPAY_KEY_SECRET"); keySecret != "" {
		cfg.Razorpay.KeySecret = keySecret
	}
	if webhookSecret := os.Getenv("RAZORPAY_WEBHOOK_SECRET"); webhookSecret != "" {
		cfg.Razorpay.WebhookSecret = webhookSecret
	}

	return &cfg
}

// fetchJWTSecretFromR2 fetches JWT secret from R2 backup for disaster recovery
func fetchJWTSecretFromR2() string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			R2AccessKey,
			R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(R2Region),
	)
	if err != nil {
		log.Printf("[Config] Failed to configure R2 client: %v", err)
		return ""
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(R2Endpoint)
	})

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(R2BucketName),
		Key:    aws.String("config/jwt_secret.txt"),
	})
	if err != nil {
		log.Printf("[Config] Failed to fetch JWT secret from R2: %v", err)
		return ""
	}
	defer result.Body.Close()

	secret, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("[Config] Failed to read JWT secret: %v", err)
		return ""
	}

	return string(secret)
}
