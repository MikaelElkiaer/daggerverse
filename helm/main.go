package main

import (
	"context"
	"fmt"
	"strings"
)

var workDir = "/src"

type Helm struct {
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
	// +default="gh"
	// +optional
	// Github username to use for OCI registry login
	githubUsername string,
) *Helm {
	return &Helm{
		AdditionalCAs:  additionalCAs,
		GithubToken:    githubToken,
		GithubUsername: githubUsername,
	}
}

type HelmPackage struct {
	Container *Container
	Output    *File
}

// Build Helm package
func (m *Helm) Build(
	ctx context.Context,
	// +default=true
	enableDocs bool,
	// +default=true
	enableLint bool,
	// +default=true
	enableSchema bool,
	// +default=true
	enableUnittest bool,
	// Local directory containing source files
	source *Directory,
) (*HelmPackage, error) {
	c := m.base()

	helmIgnoreFile, error := source.File(".helmignore").Contents(ctx)
	if error != nil {
		return nil, error
	}
	helmIgnore := strings.Split(helmIgnoreFile, "\n")

	c = c.
		WithWorkdir(workDir).
		WithDirectory(".", source.Directory("/"), ContainerWithDirectoryOpts{Include: []string{"Chart.lock", "Chart.yaml"}}).
		WithExec([]string{"sh", "-c", "touch Chart.lock && yq --indent 0 '.dependencies | map(select(.repository | test(\"^https?://\")) | [\"helm\", \"repo\", \"add\", .name, .repository] | join(\" \")) | .[]' ./Chart.lock | sh --;"}).
		WithExec([]string{"helm", "dependency", "build"}).
		WithDirectory(".", source, ContainerWithDirectoryOpts{Exclude: helmIgnore}).
		WithDirectory("templates", source.Directory("templates"))

	if enableLint {
		c = c.WithExec([]string{"helm", "lint"})
	}

	chartType, error := c.WithExec([]string{"sh", "-c", "yq '.type' Chart.yaml"}).Stdout(ctx)
	if error != nil {
		return nil, error
	}

	if strings.TrimSpace(chartType) != "library" {
		if enableSchema {
			c = c.WithExec([]string{"helm-schema"})
		}

		if enableDocs {
			c = c.WithExec([]string{"/root/go/bin/helm-docs"})
		}

		if enableUnittest {
			c = c.
				WithDirectory(".", source, ContainerWithDirectoryOpts{Include: []string{"**/*_test.yaml"}}).
				WithExec([]string{"helm", "unittest", "."})
		}
	}

	c = c.WithExec([]string{"sh", "-c", "rm -f *.tgz"}).
		WithExec([]string{"helm", "package", "."}).
		WithExec([]string{"sh", "-c", "mv *.tgz package.tgz"})

	return &HelmPackage{Container: c}, nil
}

func (hp *HelmPackage) Directory(
	ctx context.Context,
	// +default=["Chart.lock", "Chart.yaml", "package.tgz", "README.md", "values.schema.json"]
	// Files to include when exporting
	include []string,
) (*Directory, error) {
	c := dag.Container().
		WithDirectory("/src", hp.Container.Directory(workDir), ContainerWithDirectoryOpts{Include: include}).
		Directory("/src")

	return c, nil
}

func (hp *HelmPackage) List(
	ctx context.Context,
) (string, error) {
	return hp.Container.WithExec([]string{"ls", workDir}).Stdout(ctx)
}

func (hp *HelmPackage) File(
	ctx context.Context,
) (*File, error) {
	return hp.Output, nil
}

func (hp *HelmPackage) Noop() {
}

func (hp *HelmPackage) Push(
	ctx context.Context,
	// Registry URI to push the Helm package
	registry string,
) error {
	hp.Container.
		WithExec([]string{"helm", "push", "package.tgz", registry})

	return nil
}

func (hp *HelmPackage) Test(
	ctx context.Context,
	// Additional arguments to pass to helm upgrade
	// +default=""
	additionalArgs string,
	// Port to use for the Kubernetes API
	// +default=6443
	kubernetesPort int,
	// Service providing Kubernetes API
	// TODO: Make this optional and default to a built-in service
	kubernetesService *Service,
	// kubeconfig to use for Kubernetes API access
	// Required if kubernetesService is provided
	// +optional
	kubeconfig *File,
	// Name of the Helm release
	name string,
	// Namespace of the Helm release
	namespace string,
) *Container {
	return hp.Container.
		WithServiceBinding("kubernetes", kubernetesService).
		WithFile("/root/.kube/config", kubeconfig).
		WithExec([]string{"kubectl", "config", "set-cluster", "minikube", fmt.Sprintf("--server=https://kubernetes:%d", kubernetesPort)}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("helm upgrade %s ./package.tgz --atomic --create-namespace --install --namespace %s --wait %s", name, namespace, additionalArgs)}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("helm uninstall %s --namespace %s --wait", name, namespace)}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("kubectl delete namespace %s", namespace)})
}

func (m *Helm) base() *Container {
	c := dag.Container().
		// TODO: Actually implement function to update the version
		// @version policy=^3.0.0 resolved=3.19.1
		From("docker.io/library/alpine@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b").
		WithExec([]string{"sh", "-c", "echo '@community https://dl-cdn.alpinelinux.org/alpine/edge/community' >> /etc/apk/repositories"}).
		WithExec([]string{"apk", "add", "git=2.43.4-r0", "go@community=1.22.2-r0", "helm=3.14.2-r2", "npm=10.2.5-r0", "yq=4.35.2-r4"}).
		WithExec([]string{"apk", "add", "kubectl@community=1.30.0-r1"}).
		WithExec([]string{"go", "install", "github.com/norwoodj/helm-docs/cmd/helm-docs@latest"}).
		WithExec([]string{"npm", "install", "-g", "@socialgouv/helm-schema"}).
		WithExec([]string{"helm", "plugin", "install", "https://github.com/helm-unittest/helm-unittest.git"})

	if len(m.AdditionalCAs) > 0 {
		for _, ca := range m.AdditionalCAs {
			c = c.
				WithWorkdir("/usr/local/share/ca-certificates/").
				WithExec([]string{"wget", ca})
		}
		c = c.WithExec([]string{"update-ca-certificates"})
	}

	if m.GithubToken != nil {
		c = c.
			WithSecretVariable("GH_TOKEN", m.GithubToken).
			WithExec([]string{"sh", "-c", fmt.Sprintf("echo $GH_TOKEN | helm registry login --username %s --password-stdin ghcr.io", m.GithubUsername)}).
			WithoutSecretVariable("GH_TOKEN")
	}

	return c
}
