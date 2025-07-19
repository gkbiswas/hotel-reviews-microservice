package infrastructure

import "time"

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host         string        `mapstructure:"host" json:"host" validate:"required"`
	Port         int           `mapstructure:"port" json:"port" validate:"min=1,max=65535"`
	Database     string        `mapstructure:"database" json:"database" validate:"required"`
	Username     string        `mapstructure:"username" json:"username" validate:"required"`
	Password     string        `mapstructure:"password" json:"password"`
	SSLMode      string        `mapstructure:"ssl_mode" json:"ssl_mode"`
	MaxConns     int           `mapstructure:"max_conns" json:"max_conns" validate:"min=1"`
	MinConns     int           `mapstructure:"min_conns" json:"min_conns" validate:"min=1"`
	ConnTTL      time.Duration `mapstructure:"conn_ttl" json:"conn_ttl"`
	QueryTimeout time.Duration `mapstructure:"query_timeout" json:"query_timeout"`
}
