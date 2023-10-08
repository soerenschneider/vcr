package main

import (
	"vcr/internal/dbs"
	"vcr/internal/runtime"
	"vcr/internal/runtime/docker"
)

func buildRuntime() (runtime.ContainerRuntime, error) {
	return docker.NewDockerClient()
}

func buildDb() (dbs.Db, error) {
	return dbs.NewMemoryDb(), nil
}
