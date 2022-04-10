package lumigotracer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type configTestSuite struct {
	suite.Suite
}

func TestSetupConfSuite(t *testing.T) {
	suite.Run(t, &configTestSuite{})
}

func (conf *configTestSuite) TearDownTest() {
	os.Unsetenv("LUMIGO_TRACER_TOKEN")
	os.Unsetenv("LUMIGO_DEBUG")
	os.Unsetenv("LUMIGO_ENABLED")
}

func (conf *configTestSuite) TestConfigValidationMissingToken() {
	assert.Error(conf.T(), ErrInvalidToken, loadConfig(Config{}))
}

func (conf *configTestSuite) TestConfigEnvVariables() {
	os.Setenv("LUMIGO_TRACER_TOKEN", "token")
	os.Setenv("LUMIGO_DEBUG", "true")
	os.Setenv("LUMIGO_ENABLED", "false")

	err := loadConfig(Config{})
	assert.NoError(conf.T(), err)
	assert.Equal(conf.T(), "token", cfg.Token)
	assert.Equal(conf.T(), true, cfg.debug)
	assert.Equal(conf.T(), false, cfg.enabled)
}

func (conf *configTestSuite) TestConfigEnabledByDefault() {
	os.Setenv("LUMIGO_TRACER_TOKEN", "token")

	err := loadConfig(Config{})
	assert.NoError(conf.T(), err)
	assert.Equal(conf.T(), "token", cfg.Token)
	assert.Equal(conf.T(), false, cfg.debug)
	assert.Equal(conf.T(), true, cfg.enabled)
}
