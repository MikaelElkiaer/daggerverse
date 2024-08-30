package main

import (
	"context"
	"dagger/mikael-elkiaer/internal/dagger"
	"strings"
)

const (
	PACKAGE  = WORKDIR + "package.tgz"
	TEMPLATE = WORKDIR + "templated"
	WORKDIR  = "/src/"
)

type Helm struct {
	// Base container with tools
	Base *dagger.Container
	// +private
	Module *MikaelElkiaer
}

// Submodule for Helm
func (m *MikaelElkiaer) Helm(
	ctx context.Context,
) (*Helm, error) {
	b, error := m.helm(ctx)
	if error != nil {
		return nil, error
	}
	return &Helm{Base: b, Module: m}, nil
}

type HelmBuild struct {
	// Current state
	Container *dagger.Container
	// +private
	Base *dagger.Container
	// +private
	Source *dagger.Directory
	// +private
	Module *MikaelElkiaer
}

// Run build commands
func (m *Helm) Build(
	ctx context.Context,
	// +default=false
	skipRestore bool,
	// Directory containing source files
	source *dagger.Directory,
) (*HelmBuild, error) {
	helmIgnoreFile, error := source.File(".helmignore").Contents(ctx)
	if error != nil {
		return nil, error
	}
	helmIgnore := strings.Split(helmIgnoreFile, "\n")

	b := m.Base
	if !skipRestore {
		b = b.WithDirectory(WORKDIR, source.Directory("/"), dagger.ContainerWithDirectoryOpts{Include: []string{"Chart.lock", "Chart.yaml"}}).
			WithExec(inSh(`touch Chart.lock && yq --indent 0 '.dependencies | map(select(.repository | test("^https?://")) | ["helm", "repo", "add", .name, .repository] | join(" ")) | .[]' ./Chart.lock | sh --;`)).
			WithExec(inSh(`helm dependency build`))
	}
	b = b.WithDirectory(".", source, dagger.ContainerWithDirectoryOpts{Exclude: helmIgnore})

	c := dag.Container().
		WithDirectory(WORKDIR, b.Directory(WORKDIR))

	return &HelmBuild{Base: m.Base, Container: c, Module: m.Module, Source: source}, nil
}

// Get directory containing modified source files
func (m *HelmBuild) AsDirectory(
	ctx context.Context,
) *dagger.Directory {
	return m.Container.Directory(WORKDIR)
}

// Run helm lint
func (m *HelmBuild) Lint(
	ctx context.Context,
) *HelmBuild {
	return m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.WithExec(inSh(`helm lint`))
	}, nil)
}

// Run helm-schema (from @socialgouv)
func (m *HelmBuild) Schema(
	ctx context.Context,
) *HelmBuild {
	return m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`/root/go/bin/helm-schema`))
	}, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`go install github.com/dadav/helm-schema/cmd/helm-schema@latest`))
	})
}

// Run helm-docs (from @norwoodj)
func (m *HelmBuild) Docs(
	ctx context.Context,
) *HelmBuild {
	return m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`/root/go/bin/helm-docs`))
	}, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest`))
	})
}

// Run helm-unittest (from @helm-unittest)
func (m *HelmBuild) Unittest(
	ctx context.Context,
) *HelmBuild {
	return m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.
			WithDirectory(".", m.Source, dagger.ContainerWithDirectoryOpts{Include: []string{"**/*_test.yaml"}}).
			WithExec(inSh(`/root/go/bin/helm-unittest .`))
	}, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`git clone https://github.com/mikaelelkiaer/helm-unittest.git --depth=1 /tmp/helm-unittest && cd /tmp/helm-unittest/cmd/helm-unittest && go install`))
	})
}

// Run all checks
func (m *HelmBuild) Check(
	ctx context.Context,
) (*HelmBuild, error) {
	chartType, error := m.Base.
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR)).
		WithExec(inSh(`yq '.type' Chart.yaml`)).Stdout(ctx)
	if error != nil {
		return nil, error
	}

	if chartType == "library" {
		return m, nil
	}

	return m.Schema(ctx).
		Lint(ctx).
		Docs(ctx).
		Unittest(ctx), nil
}

// Package Helm chart
func (m *HelmBuild) Package(
	ctx context.Context,
) *HelmPackage {
	b := m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`helm package .`)).
			WithExec(inSh(`mv *.tgz %s`, PACKAGE))
	}, nil)

	return &HelmPackage{Base: m.Base, Container: b.Container, Module: m.Module}
}

// Template Helm chart using source
func (m *HelmBuild) Template(
	ctx context.Context,
	// Additional arguments to pass to helm template
	// +default=""
	additionalArgs string,
) *HelmTemplate {
	return template(m.Base, m.Container, ".", additionalArgs)
}

func (m *HelmBuild) run(
	ctx context.Context,
	execute func(*dagger.Container) *dagger.Container,
	setup func(*dagger.Container) *dagger.Container,
) *HelmBuild {
	b := m.Base
	if setup != nil {
		b = setup(b)
	}
	b = b.WithDirectory(WORKDIR, m.Container.Directory(WORKDIR))
	b = execute(b)
	c := dag.Container().
		WithDirectory(WORKDIR, b.Directory(WORKDIR))
	return &HelmBuild{
		Container: c,
		Base:      m.Base,
		Source:    m.Source,
	}
}

func template(base *dagger.Container, container *dagger.Container, path string, additionalArgs string) *HelmTemplate {
	b := &HelmBuild{Base: base, Container: container}
	b = b.run(context.Background(), func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`helm template %s --output-dir=%s %s`, path, TEMPLATE, additionalArgs))
	}, nil)
	return &HelmTemplate{Base: base, Container: b.Container}
}

type HelmPackage struct {
	// Current state
	Container *dagger.Container
	// +private
	Base *dagger.Container
	// +private
	Module *MikaelElkiaer
}

// Get Helm package
func (m *HelmPackage) AsFile(
	ctx context.Context,
) *dagger.File {
	return m.Container.File(PACKAGE)
}

// Push Helm package to registry
func (m *HelmPackage) Push(
	ctx context.Context,
	// Registry URI to push the Helm package
	registry string,
) *HelmPackage {
	c := m.Base.
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR)).
		WithExec(inSh(`helm push %s`, PACKAGE, registry))

	return &HelmPackage{Base: m.Base, Container: c}
}

// Template Helm chart using package
func (m *HelmPackage) Template(
	ctx context.Context,
	// Additional arguments to pass to helm template
	// +default=""
	additionalArgs string,
) *HelmTemplate {
	return template(m.Base, m.Container, PACKAGE, additionalArgs)
}

// Deploy Helm package to a cluster
func (m *HelmPackage) Deploy(
	ctx context.Context,
	// Additional arguments to pass to helm upgrade
	// +default=""
	additionalArgs string,
	// Port to use for the Kubernetes API
	// +default=8443
	kubernetesPort int,
	// Service providing Kubernetes API
	// +optional
	kubernetesService *dagger.Service,
	// kubeconfig to use for Kubernetes API access
	// Required if kubernetesService is provided
	// +optional
	kubeconfig *dagger.File,
	// Name of the Helm release
	name string,
	// Namespace of the Helm release
	namespace string,
) (*HelmPackage, error) {
	c := m.Base.
		WithFile(PACKAGE, m.AsFile(ctx))

	if kubernetesService == nil {
		k3s := dag.K3S("test")
		cluster := k3s.Server()
		cluster, err := cluster.Start(ctx)
		if err != nil {
			return nil, err
		}
		c = c.WithFile("/root/.kube/config", k3s.Config())
	} else {
		c = c.WithServiceBinding("kubernetes", kubernetesService).
			WithFile("/root/.kube/config", kubeconfig).
			WithExec(inSh(`kubectl config set-cluster minikube --server=https://kubernetes:%d`, kubernetesPort))
	}

	c = c.WithExec(inSh(`kubectl create namespace %s --dry-run=client --output=json | kubectl apply -f -`, namespace))
	c = withDockerPullSecrets(c, m.Module.Creds, namespace)
	c = c.WithExec(inSh(`helm upgrade %s %s --atomic --debug --install --namespace=%s --timeout=120s --wait %s`, name, PACKAGE, namespace, additionalArgs)).
		WithExec(inSh(`helm uninstall %s --namespace %s --wait`, name, namespace)).
		WithExec(inSh(`kubectl delete namespace %s`, namespace))

	return &HelmPackage{Base: m.Base, Container: c, Module: m.Module}, nil
}

type HelmTemplate struct {
	// Current state
	Container *dagger.Container
	// +private
	Base *dagger.Container
}

// Run kubectl-validate
func (m *HelmTemplate) Validate(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) *HelmTemplate {
	return m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`/root/go/bin/kubectl-validate %s --version %s`, TEMPLATE, kubernetesVersion))
	}, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`go install sigs.k8s.io/kubectl-validate@v0.0.4`))
	})
}

// Run pluto (from @FairwindsOps)
func (m *HelmTemplate) Pluto(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) *HelmTemplate {
	return m.run(ctx, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`pluto detect-files --target-versions k8s=v%s --v 7 --directory %s`, kubernetesVersion, TEMPLATE))
	}, func(c *dagger.Container) *dagger.Container {
		return c.
			WithExec(inSh(`wget https://github.com/FairwindsOps/pluto/releases/download/v5.19.4/pluto_5.19.4_linux_amd64.tar.gz -O pluto.tgz && tar -zxvf pluto.tgz pluto && mv pluto /usr/bin/pluto && rm pluto.tgz`))
	})
}

// Run all checks
func (m *HelmTemplate) Check(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) *HelmTemplate {
	return m.
		Validate(ctx, kubernetesVersion).
		Pluto(ctx, kubernetesVersion)
}

// Package Helm chart
func (m *HelmTemplate) Package(
	ctx context.Context,
) *HelmPackage {
	b := &HelmBuild{Base: m.Base, Container: m.Container}
	c := b.Package(ctx).Container

	return &HelmPackage{Base: m.Base, Container: c}
}

func (m *HelmTemplate) run(
	ctx context.Context,
	execute func(*dagger.Container) *dagger.Container,
	setup func(*dagger.Container) *dagger.Container,
) *HelmTemplate {
	b := m.Base
	if setup != nil {
		b = setup(b)
	}
	b = b.WithDirectory(WORKDIR, m.Container.Directory(WORKDIR))
	b = execute(b)
	c := dag.Container().
		WithDirectory(WORKDIR, b.Directory(WORKDIR))
	return &HelmTemplate{
		Base:      m.Base,
		Container: c,
	}
}

func (m *MikaelElkiaer) helm(
	ctx context.Context,
) (*dagger.Container, error) {
	c := dag.Container().
		// TODO: Actually implement function to update the version
		// @version policy=^3.0.0 resolved=3.19.1
		From("docker.io/library/alpine@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b").
		WithExec(inSh(`echo '@community https://dl-cdn.alpinelinux.org/alpine/edge/community' >> /etc/apk/repositories`)).
		WithExec(inSh(`apk add git go@community kubectl@community helm@community npm@community yq-go@community`))

	if len(m.AdditionalCAs) > 0 {
		for _, ca := range m.AdditionalCAs {
			name, error := ca.Name(ctx)
			if error != nil {
				return nil, error
			}
			c = c.
				WithWorkdir("/usr/local/share/ca-certificates/").
				WithMountedSecret(name, ca)
		}
		c = c.WithExec(inSh(`update-ca-certificates`))
	}

	for _, cred := range m.Creds {
		c = c.
			WithEnvVariable("__URL", cred.Url).
			WithEnvVariable("__USERNAME", cred.UserId).
			WithSecretVariable("__PASSWORD", cred.UserSecret).
			WithExec(inSh(`echo $__PASSWORD | helm registry login --username $__USERNAME --password-stdin $__URL`)).
			WithoutSecretVariable("__PASSWORD").
			WithoutEnvVariable("__USERNAME").
			WithoutEnvVariable("__URL")
	}

	c = c.WithWorkdir(WORKDIR)

	return c, nil
}

func withDockerPullSecrets(
	container *dagger.Container,
	creds []*Cred,
	namespace string,
) *dagger.Container {
	c := container
	for _, cred := range creds {
		c = c.
			WithEnvVariable("__NAME", cred.Name).
			WithEnvVariable("__URL", cred.Url).
			WithEnvVariable("__USERNAME", cred.UserId).
			WithSecretVariable("__PASSWORD", cred.UserSecret).
			WithExec(inSh(`kubectl --namespace %s create secret docker-registry "${__NAME}" --docker-username="${__USERNAME}" --docker-password="${__PASSWORD}" --docker-email="" --docker-server="${__URL}" --dry-run=client --output=json | kubectl apply -f -`, namespace)).
			WithoutSecretVariable("__PASSWORD").
			WithoutEnvVariable("__USERNAME").
			WithoutEnvVariable("__URL").
			WithoutEnvVariable("__NAME")
	}
	return c
}
