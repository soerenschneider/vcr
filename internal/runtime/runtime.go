package runtime

import (
	"context"
	"errors"
	"vcr/internal/config"
)

var ErrContainerNotFound = errors.New("container not found")

type ContainerRuntime interface {
	Pull(ctx context.Context, image string) error
	Run(ctx context.Context, name string, conf config.ContainerConfig) (string, error)
	FindByName(name string) (string, error)
	DeleteContainer(ctx context.Context, id string) error
	KillContainer(ctx context.Context, id string) error
}
