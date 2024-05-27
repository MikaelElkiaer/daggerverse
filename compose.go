package main

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed assets/update_images.bash
var script string

type Compose struct {
	// +private
	Main *MikaelElkiaer
}

// Docker Compose utilities
func (m *MikaelElkiaer) Compose(
	ctx context.Context,
) *Compose {
	return &Compose{Main: m}
}

// Update image references for all services
func (m *Compose) UpdateImages(
	ctx context.Context,
	file *File,
) (*File, error) {
	c := dag.Container().
		From("docker.io/library/alpine:3.19.1").
		WithExec([]string{
			"apk", "add",
			"bash=5.2.21-r0",
			"npm=10.2.5-r0",
			"skopeo=1.14.0-r2",
			"yq=4.35.2-r3",
		}).
		WithExec([]string{"npm", "install", "--global", "semver@7.6.2"})

	for _, cred := range m.Main.Creds {
		c = c.WithRegistryAuth("ghcr.io", cred.UserId, cred.UserSecret).
			WithSecretVariable("GH_TOKEN", cred.UserSecret).
			WithExec([]string{"sh", "-c", fmt.Sprintf("echo $GH_TOKEN | skopeo login --username %s --password-stdin ghcr.io", cred.UserId)}).
			WithoutSecretVariable("GH_TOKEN")
	}

	c = c.WithNewFile("script.bash", ContainerWithNewFileOpts{Contents: script}).
		WithFile("docker-compose.yaml", file).
		WithExec([]string{"bash", "script.bash"})

	f := c.File("docker-compose.yaml")

	return f, nil
}
