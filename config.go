package lumigotracer

import (
	"github.com/spf13/viper"
)

// Config describes the struct about the configuration
// of the wrap handler for tracer
type Config struct {
	// enabled switch off SDK completely
	enabled bool

	// Token is used to interact with Lumigo API
	Token string

	// debug log everything
	debug bool

	// PrintStdout prints in stdout
	PrintStdout bool

	// Maximium size for request body, request header, response body and response header
	MaxEntrySize int

	// MaxSizeForRequest is the maximum amount of byte to be sent to the edge
	MaxSizeForRequest int
}

// cfg it's a public empty config
var cfg Config

// validate runs a validation to the required fields
// for this Config struct
func (cfg Config) validate() error { // nolint
	if cfg.Token == "" {
		return ErrInvalidToken
	}
	return nil
}

// init not really used right now
func init() {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("LUMIGO")
	viper.SetDefault("ENABLED", true)
	viper.SetDefault("DEBUG", false)
}

func loadConfig(conf Config) error {
	defer recoverWithLogs()

	cfg.Token = viper.GetString("TRACER_TOKEN")
	if cfg.Token == "" {
		cfg.Token = conf.Token
	}
	cfg.enabled = viper.GetBool("ENABLED")
	cfg.debug = viper.GetBool("DEBUG")
	cfg.MaxSizeForRequest = viper.GetInt("MAX_SIZE_FOR_REQUEST")
	if cfg.MaxSizeForRequest == 0 {
		cfg.MaxSizeForRequest = 1024 * 500
	}
	cfg.MaxEntrySize = viper.GetInt("DEFAULT_MAX_ENTRY_SIZE")
	if cfg.MaxEntrySize == 0 {
		cfg.MaxEntrySize = 2048
	}
	cfg.PrintStdout = conf.PrintStdout
	return cfg.validate()
}
