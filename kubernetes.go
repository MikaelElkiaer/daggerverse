package main

import (
	_ "embed"
	"fmt"
)

//go:embed assets/kind_config.yaml
var kind_config string

//go:embed assets/kind_entrypoint.sh
var kind_entrypoint string

func minikube() *Container {
	podmanService := podman(1337, "quay.io/podman/stable", "latest", []string{"docker.io/library/alpine:latest"})

	return dag.Container().
		From("docker.io/library/alpine:3.19.1").
		WithExec([]string{"sh", "-c", "echo -e '@community https://dl-cdn.alpinelinux.org/alpine/edge/community\n@testing https://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories"}).
		WithExec([]string{"apk", "add", "fuse-overlayfs@community=1.13-r0", "minikube@testing=1.32.0-r3", "openrc@community=0.52.1-r2", "podman@community=5.0.3-r0", "sudo@community=1.9.15_p5-r0"}).
		WithServiceBinding("podman", podmanService).
		WithEnvVariable("DOCKER_HOST", "tcp://podman:1337").
		WithExec([]string{"sh", "-c", "rc-update add cgroups"}).
		WithExec([]string{"minikube", "start", "--driver=podman", "--force"}, ContainerWithExecOpts{InsecureRootCapabilities: true}).
		WithExposedPort(8443)
}

func kind() *Container {
	return dag.Container().
		From("docker.io/library/docker:dind").
		WithEnvVariable("DOCKER_TLS_CERTDIR", "/certs").
		WithMountedCache("/certs/client", dag.CacheVolume("certs-client")).
		WithExec([]string{"sh", "-c", "echo -e '@community https://dl-cdn.alpinelinux.org/alpine/edge/community\n@testing https://dl-cdn.alpinelinux.org/alpine/edge/testing' >> /etc/apk/repositories"}).
		WithExec([]string{"apk", "add", "kind@testing=0.22.0-r3"}).
		WithNewFile("/kind_config.yaml", ContainerWithNewFileOpts{Contents: kind_config}).
		WithNewFile("/kind-entrypoint.sh", ContainerWithNewFileOpts{Contents: kind_entrypoint}).
		WithExec([]string{"chmod", "+x", "/kind-entrypoint.sh"}).
		WithExec([]string{"/kind-entrypoint.sh"}, ContainerWithExecOpts{InsecureRootCapabilities: true}).
		WithExposedPort(6443)
}

func dind() *Container {
	return dag.Container().
		From("docker.io/library/docker:dind").
		WithEnvVariable("DOCKER_TLS_CERTDIR", "/certs").
		WithMountedCache("/certs/ca", dag.CacheVolume("certs-ca")).
		WithMountedCache("/certs/client", dag.CacheVolume("certs-client")).
		WithExec([]string{"dockerd-entrypoint.sh"}, ContainerWithExecOpts{InsecureRootCapabilities: true}).
		WithExposedPort(2376)
}

func podman(
	// The port to expose the podman service on
	// +optional
	// +default=1337
	port int,

	// Podman image to use
	// +optional
	// +default="quay.io/podman/stable"
	image string,

	// Podman tag to use
	// +optional
	// +default="latest"
	tag string,

	// List of images to pull
	// +optional
	pullImage []string,
) *Service {
	ctr := dag.Container().From(fmt.Sprintf("%s:%s", image, tag))

	// Install images
	for _, img := range pullImage {
		ctr = ctr.WithExec([]string{"podman", "pull", img}, ContainerWithExecOpts{InsecureRootCapabilities: true})
	}

	return ctr.WithExec([]string{"podman", "system", "service", fmt.Sprintf("tcp://0.0.0.0:%d", port), "--time", "0"}, ContainerWithExecOpts{InsecureRootCapabilities: true}).WithExposedPort(port).AsService()
}
