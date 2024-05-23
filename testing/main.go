package main

import (
	"context"
	_ "embed"
	"time"
)

type Testing struct{}

//go:embed interrupt.sh
var interrupt__sh string

func (m *Testing) Interrupt(
	ctx context.Context,
) (string, error) {
	return dag.Container().
		From("docker.io/library/alpine:3.19.1").
		WithNewFile("/interrupt.sh", ContainerWithNewFileOpts{Contents: interrupt__sh}).
		WithEnvVariable("CACHE_BUST", time.Now().String()).
		WithExec([]string{"sh", "/interrupt.sh"}).
		Stdout(ctx)
}
