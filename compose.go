package main

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed assets/update_images.bash
var script string

type Compose struct {
	Main *MikaelElkiaer
}

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

	if m.Main.GithubToken != nil {
		c = c.WithRegistryAuth("ghcr.io", m.Main.GithubUsername, m.Main.GithubToken).
			WithSecretVariable("GH_TOKEN", m.Main.GithubToken).
			WithExec([]string{"sh", "-c", fmt.Sprintf("echo $GH_TOKEN | skopeo login --username %s --password-stdin ghcr.io", m.Main.GithubUsername)}).
			WithoutSecretVariable("GH_TOKEN")
	}

	c = c.WithNewFile("script.bash", ContainerWithNewFileOpts{Contents: script}).
		WithFile("docker-compose.yaml", file).
		WithExec([]string{"bash", "script.bash"})

	f := c.File("docker-compose.yaml")

	return f, nil
}
