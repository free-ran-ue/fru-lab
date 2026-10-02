package config

import (
	"fmt"
	"time"
)

type Config struct {
	Backend BackendIE `yaml:"backend" valid:"required"`
	Logger  LoggerIE  `yaml:"logger" valid:"required"`
}

type BackendIE struct {
	Username string `yaml:"username" valid:"required"`
	Password string `yaml:"password" valid:"required"`

	Port int `yaml:"port" valid:"required"`

	JWT JWTIE `yaml:"jwt" valid:"required"`

	Db DbIE `yaml:"db" valid:"required"`

	Deploy DeployIE `yaml:"deploy" valid:"required"`

	FrontendFilePath string `yaml:"frontendFilePath" valid:"required"`

	Tester TesterIE `yaml:"tester"`
}

// TesterIE points at the fru-tester engine. Leaving URL empty disables the
// Throughput Tester pages' backend routes (they answer 503).
type TesterIE struct {
	URL      string `yaml:"url"`
	ApiToken string `yaml:"apiToken"`
}

type JWTIE struct {
	Secret    string        `yaml:"secret" valid:"required"`
	ExpiresIn time.Duration `yaml:"expiresIn" valid:"required"`
}

type DbIE struct {
	Type string `yaml:"type" valid:"required"`
	Path string `yaml:"path"`
}

type DeployIE struct {
	WorkDir string `yaml:"workDir" valid:"required"`
	// Timeout bounds one deploy (0 = 5m). A deploy that fails or runs over
	// is taken down again and the error is shown.
	Timeout time.Duration `yaml:"timeout"`
	// Webconsole is where free5GC's webconsole is published on the host.
	Webconsole WebconsoleIE `yaml:"webconsole"`
}

// WebconsoleIE says where fru-lab publishes the webconsole: on Port (0 = 5000), and
// reaches it at Host:Port (Host empty = localhost; host.docker.internal
// when fru-lab itself runs in a container). The browser never talks to
// the webconsole directly: fru-lab proxies it under /api/webconsole.
type WebconsoleIE struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

func (d DeployIE) WebconsolePort() int {
	if d.Webconsole.Port == 0 {
		return 5000
	}
	return d.Webconsole.Port
}

func (d DeployIE) WebconsoleURL() string {
	host := d.Webconsole.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, d.WebconsolePort())
}

func (d DeployIE) DeployTimeout() time.Duration {
	if d.Timeout == 0 {
		return 5 * time.Minute
	}
	return d.Timeout
}

type LoggerIE struct {
	Level string `yaml:"level" valid:"required"`
}
