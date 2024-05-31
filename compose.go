package main

import (
	"context"
	_ "embed"
)

//go:embed assets/update_images.bash
var script string

type Compose struct {
	// Current state
	Container *Container
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
	file *File,
) *File {
	c := m.Container.
		WithNewFile("script.bash", ContainerWithNewFileOpts{Contents: script}).
		WithFile("docker-compose.yaml", file).
		WithExec(inSh("bash", "script.bash"))

	f := c.File("docker-compose.yaml")

	return f
}

func (m *MikaelElkiaer) compose(
	ctx context.Context,
) *Container {
	c := dag.Container().
		From("docker.io/library/alpine:3.19.1").
		WithExec(inSh("apk add bash=5.2.21-r0 npm=10.2.5-r0 skopeo=1.14.0-r2 yq=4.35.2-r3")).
		WithExec(inSh("npm install --global semver@7.6.2"))

	for _, cred := range m.Creds {
		c = c.WithRegistryAuth("ghcr.io", cred.UserId, cred.UserSecret).
			WithSecretVariable("GH_TOKEN", cred.UserSecret).
			WithExec(inSh("echo $GH_TOKEN | skopeo login --username %s --password-stdin ghcr.io", cred.UserId)).
			WithoutSecretVariable("GH_TOKEN")
	}

	return c
}
