package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
	"vcr/internal/config"
	"vcr/internal/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

const VcrLabelNameKey = "vcr_name"

type Docker struct {
	client *client.Client
}

func NewDockerClient() (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.WithHostFromEnv(), client.WithVersion("1.41"))
	if err != nil {
		return nil, err
	}

	docker := &Docker{
		client: cli,
	}

	return docker, nil
}

func (d *Docker) Pull(ctx context.Context, image string) error {
	opts := types.ImagePullOptions{}

	events, err := d.client.ImagePull(ctx, image, opts)
	if err != nil {
		return err
	}
	defer events.Close()

	decode := json.NewDecoder(events)

	type Event struct {
		Status         string `json:"status"`
		Error          string `json:"error"`
		Progress       string `json:"progress"`
		ProgressDetail struct {
			Current int `json:"current"`
			Total   int `json:"total"`
		} `json:"progressDetail"`
	}

	var event *Event
	for {
		if err := decode.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	return nil
}

func translateVolumes(conf config.ContainerConfig) map[string]struct{} {
	output := map[string]struct{}{}

	if conf.Mount == nil {
		return nil
	}

	t := fmt.Sprintf("%s:%s", conf.Mount.HostPath, conf.Mount.ContainerPath)
	output[t] = struct{}{}
	return output
}

func (d *Docker) Run(ctx context.Context, name string, conf config.ContainerConfig) (string, error) {
	containerConfig := &container.Config{
		Image:   conf.Image,
		Volumes: translateVolumes(conf),
		Cmd:     conf.Args,
		Labels: map[string]string{
			VcrLabelNameKey: name,
			"app":           "vcr",
		},
	}

	hostConf := &container.HostConfig{}
	if conf.Mount != nil && len(conf.Mount.HostPath) > 0 {
		hostConf.Mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: conf.Mount.HostPath,
				Target: conf.Mount.ContainerPath,
			},
		}
	}

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConf, nil, nil, "")
	if err != nil {
		return "", err
	}
	opt := types.ContainerStartOptions{}

	return resp.ID, d.client.ContainerStart(ctx, resp.ID, opt)
}

func (d *Docker) FindByName(name string) (string, error) {
	filter := []filters.KeyValuePair{
		{
			Key:   "label",
			Value: fmt.Sprintf("%s=%s", VcrLabelNameKey, name),
		},
	}

	args := filters.NewArgs(filter...)
	containersList, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{
		All:     true,
		Latest:  true,
		Limit:   1,
		Filters: args,
	})
	if err != nil {
		return "", err
	}

	if len(containersList) > 0 {
		return containersList[0].ID, nil
	}

	return "", runtime.ErrContainerNotFound
}

func (d *Docker) DeleteContainer(ctx context.Context, id string) error {
	opts := types.ContainerRemoveOptions{
		RemoveVolumes: false,
		RemoveLinks:   false,
		Force:         false,
	}
	return d.client.ContainerRemove(ctx, id, opts)

}

func (d *Docker) KillContainer(ctx context.Context, id string) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := d.client.ContainerKill(ctxTimeout, id, "SIGKILL")
	if err != nil {
		log.Error().Err(err).Msgf("could not kill container %s", id)
		return err
	}
	log.Info().Msgf("Container %s killed", id)
	return nil
}
