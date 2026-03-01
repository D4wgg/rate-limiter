package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Addr         string        `mapstructure:"addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type LimitConfig struct {
	RPS    int           `mapstructure:"rps"`
	Window time.Duration `mapstructure:"window"`
}

type RouteConfig struct {
	Route    string      `mapstructure:"route"`
	Methods  []string    `mapstructure:"methods"`
	Upstream string      `mapstructure:"upstream"`
	Limit    LimitConfig `mapstructure:"limit"`
}

type Config struct {
	Server ServerConfig  `mapstructure:"server"`
	Routes []RouteConfig `mapstructure:"routes"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("RATE_LIMITER")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 5 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 10 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 60 * time.Second
	}

	// Валидация конфигурации
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate проверяет корректность конфигурации.
func (c *Config) Validate() error {
	if len(c.Routes) == 0 {
		return fmt.Errorf("at least one route must be configured")
	}

	for i, rt := range c.Routes {
		if rt.Route == "" {
			return fmt.Errorf("route[%d]: route path cannot be empty", i)
		}

		if rt.Upstream == "" {
			return fmt.Errorf("route[%d]: upstream URL cannot be empty", i)
		}

		upstreamURL, err := url.Parse(rt.Upstream)
		if err != nil {
			return fmt.Errorf("route[%d]: invalid upstream URL: %w", i, err)
		}

		if upstreamURL.Scheme != "http" && upstreamURL.Scheme != "https" {
			return fmt.Errorf("route[%d]: upstream URL must use http or https scheme", i)
		}

		if rt.Limit.RPS <= 0 {
			return fmt.Errorf("route[%d]: limit.rps must be positive", i)
		}

		if rt.Limit.Window <= 0 {
			return fmt.Errorf("route[%d]: limit.window must be positive", i)
		}

		// Проверка методов
		for j, method := range rt.Methods {
			if method == "" {
				return fmt.Errorf("route[%d]: methods[%d] cannot be empty", i, j)
			}
		}
	}

	return nil
}
