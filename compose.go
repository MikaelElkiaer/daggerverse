package main

import (
	"context"
	"dagger/mikael-elkiaer/internal/dagger"
	_ "embed"
)

//go:embed assets/update_images.bash
var script string

type Compose struct {
	// Current state
	Container *dagger.Container
}

// Submodule for Docker Compose
func (m *MikaelElkiaer) Compose(
	ctx context.Context,
) *Compose {
	c := m.compose(ctx)
	return &Compose{Container: c}
}

// Update image tags for all services
func (m *Compose) UpdateImages(
	ctx context.Context,
	// Docker Compose file
	file *dagger.File,
) *dagger.File {
	c := m.Container.
		WithNewFile("script.bash", script).
		WithFile("docker-compose.yaml", file).
		WithExec(inSh("bash", "script.bash"))

	f := c.File("docker-compose.yaml")

	return f
}

func (m *MikaelElkiaer) compose(
	ctx context.Context,
) *dagger.Container {
	c := dag.Container().
		From("docker.io/library/alpine:3.21.3@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c").
		WithExec(inSh("apk add bash=5.2.21-r0 npm=10.2.5-r0 skopeo=1.14.0-r2 yq=4.35.2-r3")).
		WithExec(inSh("npm install --global semver@7.6.2"))

	for _, cred := range m.Creds {
		c = c.WithRegistryAuth("ghcr.io", cred.UserId, cred.UserSecret).
			WithEnvVariable("__URL", cred.Url).
			WithEnvVariable("__USERNAME", cred.UserId).
			WithSecretVariable("__PASSWORD", cred.UserSecret).
			WithExec(inSh("echo $__PASSWORD | skopeo login --username $__USERNAME --password-stdin $__URL")).
			WithoutSecretVariable("__PASSWORD").
			WithoutEnvVariable("__USERNAME").
			WithoutEnvVariable("__URL")
	}

	return c
}
