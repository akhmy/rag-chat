package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type DBConfig struct {
	User         string   `env:"DB_USER,required"`
	PasswordFile string   `env:"DB_PASSWORD_FILE,required"`
	Host         string   `env:"DB_HOST,required"`
	Port         int      `env:"DB_PORT,required"`
	Name         string   `env:"DB_NAME,required"`
	Options      []string `env:"DB_OPTIONS,required"`
}

type ServerConfig struct {
	Addr              string        `env:"SERVER_ADDR" envDefault:":8080"`
	ReadTimeout       time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"5s"`
	ReadHeaderTimeout time.Duration `env:"SERVER_READ_HEADER_TIMEOUT" envDefault:"5s"`
	WriteTimeout      time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"5s"`
	IdleTimeout       time.Duration `env:"SERVER_IDLE_TIMEOUT" envDefault:"5s"`
}

type APIConfig struct {
	DB               DBConfig
	Server           ServerConfig
	OpenAIAPIKeyFile string `env:"OPENAI_API_KEY_FILE,required"`
}

type IngestConfig struct {
	DB               DBConfig
	OpenAIAPIKeyFile string `env:"OPENAI_API_KEY_FILE,required"`
}

type CLIConfig struct {
	Host string `env:"CHAT_HOST,required"`
}

func ParseEnv[T APIConfig | IngestConfig | CLIConfig]() (T, error) {
	return env.ParseAs[T]()
}
