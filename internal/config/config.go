package config

import (
	"time"

	"github.com/caarlos0/env/v9"
)

type VcrConfig struct {
	RuntimeImpl string `yaml:"runtime_impl"`

	Docker struct {
		DockerHost string `yaml:"docker_host"`
		ApiVersion string `yaml:"omitempty,api_version"`
	}

	ContainerConfig ContainerConfig `yaml:"container_config"`
}

type Programming struct {
	Url   string     `yaml:"url" validate:"required,url"`
	Name  string     `yaml:"name" validate:"required"`
	Date  time.Time  `yaml:"start" validate:"required"`
	Until *time.Time `yaml:"end,omitempty" validate:"omitempty,datetime"`
}

func (p *Programming) IsUpcoming() bool {
	return p.Date.After(time.Now())
}

func GetContainerConfig() (ContainerConfig, error) {
	conf := getDefaultConfig()
	err := env.Parse(&conf)
	return conf, err
}

func getDefaultConfig() ContainerConfig {
	return ContainerConfig{
		Image: "ghcr.io/soerenschneider/yt-dlp:main",
		Mount: &Mount{
			ContainerPath: ".",
		},
	}
}

type Mount struct {
	HostPath      string `yaml:"host_path" env:"VCR_MOUNT_HOST" validate:"omitempty,dirpath"`
	ContainerPath string `yaml:"container_path" env:"VCR_MOUNT_CONTAINER" validate:"omitempty,dirpath"`
}

type ContainerConfig struct {
	// TODO: not ends with / paths
	Image string   `yaml:"image" env:"VCR_IMAGE" validate:"required"`
	Mount *Mount   `yaml:"mount"`
	Args  []string `yaml:"args"`
}
