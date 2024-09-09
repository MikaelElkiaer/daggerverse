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
	Container *dagger.Container
	//+private
	Module  *MikaelElkiaer
  // Modified source directory
	Workdir *dagger.Directory
}

// Submodule for Helm
func (m *MikaelElkiaer) Helm(
	ctx context.Context,
  // Helm chart path
	source *dagger.Directory,
) (*Helm, error) {
	c, err := m.createHelmContainer(ctx)
	if err != nil {
		return nil, err
	}

	return &Helm{
		Container: c,
		Module:    m,
		Workdir:   source,
	}, nil
}

// Run build commands
func (m *Helm) Build(
	ctx context.Context,
) (*Helm, error) {
	c := m.Container.WithDirectory(WORKDIR, m.Workdir, dagger.ContainerWithDirectoryOpts{Include: []string{"Chart.lock", "Chart.yaml"}}).
		WithExec(inSh(`touch Chart.lock && yq --indent 0 '.dependencies | map(select(.repository | test("^https?://")) | ["helm", "repo", "add", .name, .repository] | join(" ")) | .[]' ./Chart.lock | sh --;`)).
		WithExec(inSh(`helm dependency build`)).
		WithDirectory(WORKDIR, m.Workdir)

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Run helm lint
func (m *Helm) Lint(
	ctx context.Context,
) (*Helm, error) {
	c := m.Container.WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`helm lint .`))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Run helm-schema (from @socialgouv)
func (m *Helm) Schema(
	ctx context.Context,
) (*Helm, error) {
	c := m.Container.WithExec(inSh(`go install github.com/dadav/helm-schema/cmd/helm-schema@latest`)).
		WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`/root/go/bin/helm-schema`))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Run helm-docs (from @norwoodj)
func (m *Helm) Docs(
	ctx context.Context,
) (*Helm, error) {
	c := m.Container.WithExec(inSh(`go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest`)).
		WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`/root/go/bin/helm-docs`))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Run helm-unittest (from @helm-unittest)
func (m *Helm) Unittest(
	ctx context.Context,
) (*Helm, error) {
	c := m.Container.WithExec(inSh(`git clone https://github.com/mikaelelkiaer/helm-unittest.git --depth=1 /tmp/helm-unittest && cd /tmp/helm-unittest/cmd/helm-unittest && go install`)).
		WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`/root/go/bin/helm-unittest .`))

	m.Workdir = c.Directory(WORKDIR)
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
	c := m.Container.WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`helm package .`)).
		WithExec(inSh(`mv *.tgz %s`, PACKAGE))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Template Helm chart using source
func (m *Helm) Template(
	ctx context.Context,
	// Additional arguments to pass to helm template
	// +default=""
	additionalArgs string,
) (*Helm, error) {
	c := m.Container.WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`helm template %s --output-dir=%s %s`, ".", TEMPLATEDIR, additionalArgs))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Push Helm package to registry
func (m *Helm) Push(
	ctx context.Context,
	// Registry URI to push the Helm package
	registry string,
) (*Helm, error) {
	c := m.Container.WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`helm push %s`, PACKAGE, registry))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Install Helm package to a cluster
func (m *Helm) Install(
	ctx context.Context,
	// Additional arguments to pass to helm upgrade
	// +default=""
	additionalArgs string,
	// Port to use for the Kubernetes API
	// +default=6443
	kubernetesPort int,
	// Service providing Kubernetes API
	// TODO: Make this optional and default to a built-in service
	kubernetesService *dagger.Service,
	// kubeconfig to use for Kubernetes API access
	// Required if kubernetesService is provided
	// +optional
	kubeconfig *dagger.File,
	// Name of the Helm release
	name string,
	// Namespace of the Helm release
	namespace string,
) (*Helm, error) {
	c := m.Container.WithDirectory(WORKDIR, m.Workdir).
		WithServiceBinding("kubernetes", kubernetesService).
		WithFile("/root/.kube/config", kubeconfig).
		WithExec(inSh(`kubectl config set-cluster minikube --server=https://kubernetes:%d`, kubernetesPort)).
		WithExec(inSh(`kubectl create namespace %s --dry-run=client --output=json | kubectl apply -f -`, namespace))

	c = withDockerPullSecrets(c, name, namespace)

	c = c.WithExec(inSh(`helm upgrade %s %s --atomic --install --namespace %s --wait %s`, name, PACKAGE, namespace, additionalArgs)).
		WithExec(inSh(`helm uninstall %s --namespace %s --wait`, name, namespace)).
		WithExec(inSh(`kubectl delete namespace %s`, namespace))

	return m, nil
}

// Run kubectl-validate
func (m *Helm) Validate(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) (*Helm, error) {
	c := m.Container.WithExec(inSh(`go install sigs.k8s.io/kubectl-validate@v0.0.4`)).
		WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`/root/go/bin/kubectl-validate %s --version %s`, TEMPLATEDIR, kubernetesVersion))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Run pluto (from @FairwindsOps)
func (m *Helm) Pluto(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) (*Helm, error) {
	c := m.Container.
		WithExec(inSh(`wget https://github.com/FairwindsOps/pluto/releases/download/v5.19.4/pluto_5.19.4_linux_amd64.tar.gz -O pluto.tgz && tar -zxvf pluto.tgz pluto && mv pluto /usr/bin/pluto && rm pluto.tgz`)).
		WithDirectory(WORKDIR, m.Workdir).
		WithExec(inSh(`pluto detect-files --target-versions k8s=v%s --v 7 --directory %s`, kubernetesVersion, TEMPLATEDIR))

	m.Workdir = c.Directory(WORKDIR)
	return m, nil
}

// Run all checks
func (m *Helm) CheckTemplated(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) (*Helm, error) {
	c, err := m.Template(ctx, "")
	if err != nil {
		return nil, err
	}

	c, err = c.Validate(ctx, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	c, err = c.Pluto(ctx, kubernetesVersion)
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
	name string,
	namespace string,
) *dagger.Container {
	return container.
		WithExec(inSh(`[ -f /root/.docker/config.json ] && cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
data:
  .dockerconfigjson: $(cat /root/.docker/config.json | base64 -w 0)
type: kubernetes.io/dockerconfigjson
EOF`, name, namespace))
}
