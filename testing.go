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
		From("docker.io/library/alpine:3.24.0@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4").
		WithNewFile("/interrupt.sh", interrupt__sh).
		WithEnvVariable("CACHE_BUST", time.Now().String()).
		WithExec([]string{"sh", "/interrupt.sh"}).
		Stdout(ctx)
}
