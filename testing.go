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
		From("docker.io/library/alpine:3.21.0@sha256:21dc6063fd678b478f57c0e13f47560d0ea4eeba26dfc947b2a4f81f686b9f45").
		WithNewFile("/interrupt.sh", interrupt__sh).
		WithEnvVariable("CACHE_BUST", time.Now().String()).
		WithExec([]string{"sh", "/interrupt.sh"}).
		Stdout(ctx)
}
