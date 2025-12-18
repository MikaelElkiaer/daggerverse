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
		From("docker.io/library/alpine:3.23.2@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62").
		WithNewFile("/interrupt.sh", interrupt__sh).
		WithEnvVariable("CACHE_BUST", time.Now().String()).
		WithExec([]string{"sh", "/interrupt.sh"}).
		Stdout(ctx)
}
