package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env           string              `yaml:"env" env-default:"local"`
	AppSecret     string              `yaml:"app_secret" env:"APP_SECRET"`
	TokenTTL      time.Duration       `yaml:"token_ttl" env-default:"10m"`
	HTTP          HTTPConfig          `yaml:"http"`
	AuthGRPC      AuthGRPCConfig      `yaml:"auth_grpc"`
	ScriptService ScriptServiceConfig `yaml:"script_service"`
	VideoService  VideoServiceConfig  `yaml:"video_service"`
}

type HTTPConfig struct {
	Host         string        `yaml:"host" env-default:"0.0.0.0"`
	Port         int           `yaml:"port" env-default:"8080"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env-default:"5s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env-default:"5s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

type AuthGRPCConfig struct {
	Address string        `yaml:"address" env-required:"true"`
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
}

type ScriptServiceConfig struct {
	BaseURL string        `yaml:"base_url" env-required:"true"`
	Timeout time.Duration `yaml:"timeout" env-default:"10s"`
}

type VideoServiceConfig struct {
	BaseURL string        `yaml:"base_url" env-required:"true"`
	Timeout time.Duration `yaml:"timeout" env-default:"10s"`
}

func MustLoad() *Config {
	configPath := fetchConfigPath()
	if configPath == "" {
		panic("config path is empty")
	}

	return MustLoadPath(configPath)
}

func MustLoadPath(configPath string) *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
