package main

import (
	"context"
	"dagger/mikael-elkiaer/internal/dagger"
)

const (
	PACKAGE     = WORKDIR + "package.tgz"
	TEMPLATEDIR = WORKDIR + "templated"
	WORKDIR     = "/src/"
)

type Helm struct {
	//+private
	Base *dagger.Container
	// Latest run container, contains workdir
	Container *dagger.Container
	//+private
	Module *MikaelElkiaer
	//+private
	TargetKubernetesVersion string
}

// Submodule for Helm
func (m *MikaelElkiaer) Helm(
	ctx context.Context,
	// Helm chart path
	source *dagger.Directory,
	// Kubernetes version to check against
	// +default="1.29"
	targetKubernetesVersion string,
) (*Helm, error) {
	c, err := m.createHelmContainer(ctx)
	if err != nil {
		return nil, err
	}

	return &Helm{
		Base:                    c,
		Container:               c.WithDirectory(WORKDIR, source),
		Module:                  m,
		TargetKubernetesVersion: targetKubernetesVersion,
	}, nil
}

func (m *Helm) workdir() *dagger.Directory {
	return m.Container.Directory(WORKDIR)
}

// Run build commands
func (m *Helm) Build(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.WithDirectory(WORKDIR, m.workdir(), dagger.ContainerWithDirectoryOpts{Include: []string{"Chart.lock", "Chart.yaml"}}).
		WithExec(inSh(`touch Chart.lock && yq --indent 0 '.dependencies | map(select(.repository | test("^https?://")) | ["helm", "repo", "add", .name, .repository] | join(" ")) | .[]' ./Chart.lock | sh --;`)).
		WithExec(inSh(`helm dependency build`)).
		WithDirectory(WORKDIR, m.workdir())

	return m, nil
}

// Run helm lint
func (m *Helm) Lint(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`helm lint .`))

	return m, nil
}

// Run helm-schema (from @socialgouv)
func (m *Helm) Schema(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.WithExec(inSh(`go install github.com/dadav/helm-schema/cmd/helm-schema@latest`)).
		WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`/root/go/bin/helm-schema`))

	return m, nil
}

// Run helm-docs (from @norwoodj)
func (m *Helm) Docs(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.WithExec(inSh(`go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest`)).
		WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`/root/go/bin/helm-docs`))

	return m, nil
}

// Run helm-unittest (from @helm-unittest)
func (m *Helm) Unittest(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.
		WithExec(inSh(`helm plugin install https://github.com/helm-unittest/helm-unittest.git`)).
		WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`helm unittest .`))

	return m, nil
}

// Run all checks
func (m *Helm) Check(
	ctx context.Context,
) (*Helm, error) {
	m, err := m.Build(ctx)
	if err != nil {
		return nil, err
	}
	m, err = m.Lint(ctx)
	if err != nil {
		return nil, err
	}
	m, err = m.Schema(ctx)
	if err != nil {
		return nil, err
	}
	m, err = m.Unittest(ctx)
	if err != nil {
		return nil, err
	}
	m, err = m.Docs(ctx)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Package Helm chart
func (m *Helm) Package(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`helm package .`)).
		WithExec(inSh(`mv *.tgz %s`, PACKAGE))

	return m, nil
}

// Template Helm chart using source
func (m *Helm) Template(
	ctx context.Context,
	// Additional arguments to pass to helm template
	// +default=""
	additionalArgs string,
) (*Helm, error) {
	m.Container = m.Base.WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`helm template %s --output-dir=%s %s`, ".", TEMPLATEDIR, additionalArgs))

	return m, nil
}

// Push Helm package to registry
func (m *Helm) Push(
	ctx context.Context,
	// Registry URI to push the Helm package
	registry string,
) (*Helm, error) {
	m.Container = m.Base.WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`helm push %s`, PACKAGE, registry))

	return m, nil
}

// Install Helm package to a cluster
func (m *Helm) Install(
	ctx context.Context,
	// Additional arguments to pass to helm upgrade
	// +default=""
	additionalArgs string,
	// Service providing Kubernetes API
	// +optional
	kubernetesService *dagger.Service,
	// kubeconfig to use for Kubernetes API access
	// Required if kubernetesService is provided
	// +optional
	kubeconfig *dagger.File,
	// Name of the Helm release
	// +default="test"
	name string,
	// Namespace of the Helm release
	// +default="testing"
	namespace string,
	// Containers to load into the cluster
	// +optional
	preloadContainers []*dagger.Container,
	// Tarball to load into the cluster
	// +optional
	preloadTarball *dagger.File,
	// Timeout for Helm operations
	// +default="300s"
	timeout string,
) (*Helm, error) {
	c := m.Base.WithDirectory(WORKDIR, m.workdir())

	if kubernetesService == nil {
		k3s := dag.K3S("test")
		k3s = withAdditionalCAs(k3s, m.Module.AdditionalCAs)
		k3s = withPreloadContainers(k3s, preloadContainers)
		k3s = withPreloadTarball(k3s, preloadTarball)
		cluster := k3s.Server()
		cluster, err := cluster.Start(ctx)
		if err != nil {
			return nil, err
		}
		c = c.WithFile("/root/.kube/config", k3s.Config())
	} else {
		c = c.WithServiceBinding("kubernetes", kubernetesService).
			WithFile("/root/.kube/config", kubeconfig).
			WithExec(inSh(`sed -E 's,(server: https://)(.+)(:.+)$,\1kubernetes\3,' -i /root/.kube/config`))
	}

	c = c.WithExec(inSh(`kubectl create namespace %s --dry-run=client --output=json | kubectl apply -f -`, namespace))
	c = withDockerPullSecrets(c, m.Module.Creds, namespace)
	c = c.WithExec(inSh(`helm upgrade %s %s --debug --install --namespace=%s --timeout=%s --wait %s`, name, ".", namespace, timeout, additionalArgs)).
		WithExec(inSh(`helm uninstall %s --debug --namespace %s --wait`, name, namespace)).
		WithExec(inSh(`kubectl delete namespace %s`, namespace))

	m.Container = c
	return m, nil
}

// Uninstall Helm package in a cluster
func (m *Helm) Uninstall(
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
	// +default="test"
	name string,
	// Namespace of the Helm release
	// +default="testing"
	namespace string,
) (*Helm, error) {
	c := m.Base.WithDirectory(WORKDIR, m.workdir())
	c = c.WithServiceBinding("kubernetes", kubernetesService).
		WithFile("/root/.kube/config", kubeconfig).
		WithExec(inSh(`kubectl config set-cluster minikube --server=https://kubernetes:%d`, kubernetesPort)).
		WithExec(inSh(`helm uninstall %s --debug --namespace %s --wait || true`, name, namespace)).
		WithExec(inSh(`kubectl delete namespace %s || true`, namespace))

	m.Container = c
	return m, nil
}

// Run kubectl-validate
func (m *Helm) Validate(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.
		WithExec(inSh(`go install sigs.k8s.io/kubectl-validate@v0.0.4`)).
		WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`/root/go/bin/kubectl-validate %s --version %s`, TEMPLATEDIR, m.TargetKubernetesVersion))

	return m, nil
}

// Run pluto (from @FairwindsOps)
func (m *Helm) Pluto(
	ctx context.Context,
) (*Helm, error) {
	m.Container = m.Base.
		WithExec(inSh(`wget https://github.com/FairwindsOps/pluto/releases/download/v5.19.4/pluto_5.19.4_linux_amd64.tar.gz -O pluto.tgz && tar -zxvf pluto.tgz pluto && mv pluto /usr/bin/pluto && rm pluto.tgz`)).
		WithDirectory(WORKDIR, m.workdir()).
		WithExec(inSh(`pluto detect-files --target-versions k8s=v%s --v 7 --directory %s`, m.TargetKubernetesVersion, TEMPLATEDIR))

	return m, nil
}

// Run all checks
func (m *Helm) CheckTemplated(
	ctx context.Context,
) (*Helm, error) {
	m, err := m.Template(ctx, "")
	if err != nil {
		return nil, err
	}

	m, err = m.Validate(ctx)
	if err != nil {
		return nil, err
	}

	m, err = m.Pluto(ctx)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *MikaelElkiaer) createHelmContainer(
	ctx context.Context,
) (*dagger.Container, error) {
	c := dag.Container().
		// TODO: Actually implement function to update the version
		// @version policy=^3.0.0 resolved=3.20.1
		From("docker.io/library/alpine@sha256:beefdbd8a1da6d2915566fde36db9db0b524eb737fc57cd1367effd16dc0d06d").
		WithExec(inSh(`apk add git go kubectl helm npm yq-go`))

	if len(m.AdditionalCAs) > 0 {
		for _, ca := range m.AdditionalCAs {
			name, error := ca.Name(ctx)
			if error != nil {
				return nil, error
			}
			c = c.
				WithWorkdir("/usr/local/share/ca-certificates/").
				WithFile(name, ca)
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

func withAdditionalCAs(
	k3s *dagger.K3S,
	cas []*dagger.File,
) *dagger.K3S {
	k3sContainer := k3s.Container()
	for _, ca := range cas {
		k3sContainer = k3sContainer.
			WithFile("/tmp/additional-ca.crt", ca).
			WithExec(inSh(`cat /tmp/additional-ca.crt >> /etc/ssl/certs/ca-certificates.crt`)).
			WithoutFile("/tmp/additional-ca.crt")
	}
	k3s = k3s.WithContainer(k3sContainer)
	return k3s
}

func withPreloadContainers(
	k3s *dagger.K3S,
	preloads []*dagger.Container,
) *dagger.K3S {
	k3sContainer := k3s.Container()
	for _, preload := range preloads {
		k3sContainer = k3sContainer.WithExec(inSh(`ctr --namespace k8s.io image import %s`, preload.AsTarball()))
	}
	k3s = k3s.WithContainer(k3sContainer)
	return k3s
}

func withPreloadTarball(
	k3s *dagger.K3S,
	tarball *dagger.File,
) *dagger.K3S {
	k3sContainer := k3s.Container()
	if tarball != nil {
		k3sContainer = k3sContainer.WithExec(inSh(`ctr --namespace k8s.io image import %s`, tarball))
	}
	k3s = k3s.WithContainer(k3sContainer)
	return k3s
}
