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
		From("docker.io/library/alpine:3.22.0@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715").
		WithNewFile("/interrupt.sh", interrupt__sh).
		WithEnvVariable("CACHE_BUST", time.Now().String()).
		WithExec([]string{"sh", "/interrupt.sh"}).
		Stdout(ctx)
}
