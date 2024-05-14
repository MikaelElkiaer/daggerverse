package main

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed scripts/update_images.bash
var script string

type Compose struct {
	GithubToken    *Secret
	GithubUsername string
	AdditionalCAs  []string
}

func New(
	// +optional
	// Additional CA certs to add to the running container
	additionalCAs []string,
	// +optional
	// Github token to use for OCI registry login
	githubToken *Secret,
	// +optional
	// +default="gh"
	// Github username to use for OCI registry login
	githubUsername string,
) *Compose {
	return &Compose{
		AdditionalCAs:  additionalCAs,
		GithubToken:    githubToken,
		GithubUsername: githubUsername,
	}
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

	if m.GithubToken != nil {
		c = c.WithRegistryAuth("ghcr.io", m.GithubUsername, m.GithubToken).
			WithSecretVariable("GH_TOKEN", m.GithubToken).
			WithExec([]string{"sh", "-c", fmt.Sprintf("echo $GH_TOKEN | skopeo login --username %s --password-stdin ghcr.io", m.GithubUsername)}).
			WithoutSecretVariable("GH_TOKEN")
	}

	c = c.WithNewFile("script.bash", ContainerWithNewFileOpts{Contents: script}).
		WithFile("docker-compose.yaml", file).
		WithExec([]string{"bash", "script.bash"})

	f := c.File("docker-compose.yaml")

	return f, nil
}
