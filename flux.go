package main

import "context"

type Flux struct {
	// Base container with tools
	Base *Container
}

// Submodule for FluxCD
func (m *MikaelElkiaer) Flux(
	ctx context.Context,
) (*Helm, error) {
	b, error := m.flux(ctx)
	if error != nil {
		return nil, error
	}
	return &Helm{Base: b}, nil
}

func (m *MikaelElkiaer) flux(
	ctx context.Context,
) (*Container, error) {
	return dag.Container().
		// TODO: Actually implement function to update the version
		// @version policy=^3.0.0 resolved=3.19.1
		From("docker.io/library/alpine@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b").
		WithExec(inSh(`echo '@community https://dl-cdn.alpinelinux.org/alpine/edge/community' >> /etc/apk/repositories`)).
		WithExec(inSh(`echo '@testing https://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories`)).
		WithExec(inSh(`apk add flux@testing=2.2.3-r3 yq=4.35.2-r4`)), nil
}
