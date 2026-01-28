package main

import (
	"context"
	_ "embed"
	"time"
)

type Testing struct {
	// +private
	Main *MikaelElkiaer
}

// Simple examples for test purposes
func (m *MikaelElkiaer) Testing(
	ctx context.Context,
) *Testing {
	return &Testing{Main: m}
}

//go:embed assets/interrupt.sh
var interrupt__sh string

func (m *Testing) Interrupt(
	ctx context.Context,
) (string, error) {
	return dag.Container().
		From("docker.io/library/alpine:3.23.3@sha256:eb37f58646a901dc7727cf448cae36daaefaba79de33b5058dab79aa4c04aefb").
		WithNewFile("/interrupt.sh", interrupt__sh).
		WithEnvVariable("CACHE_BUST", time.Now().String()).
		WithExec([]string{"sh", "/interrupt.sh"}).
		Stdout(ctx)
}
