package main

import (
	"context"
	"fmt"
	"strings"
)

var (
	workDir      = "/src"
	templatedDir = "/src/templated"
)

type Helm struct {
	// +private
	Main *MikaelElkiaer
}

// Helm package manager
func (m *MikaelElkiaer) Helm(
	ctx context.Context,
) *Helm {
	return &Helm{Main: m}
}

type HelmPackage struct {
	Container *Container
	Parent    *Helm
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
	c, error := m.base(ctx)
	if error != nil {
		return nil, error
	}

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
		WithDirectory(".", source, ContainerWithDirectoryOpts{Exclude: helmIgnore})

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

	return &HelmPackage{Container: c, Parent: m}, nil
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
	return hp.Container.File("package.tgz"), nil
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

func (hp *HelmPackage) Template(
	ctx context.Context,
	// Additional arguments to pass to helm template
	// +default=""
	additionalArgs string,
	// Namespace to template
	// +default="test"
	namespace string,
) (*HelmTemplated, error) {
	c := hp.Container.
		WithExec(inSh("helm template . --namespace=%s --output-dir=%s %s", namespace, templatedDir, additionalArgs))

	return &HelmTemplated{Container: c, Parent: hp}, nil
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
	c := hp.Container.
		WithServiceBinding("kubernetes", kubernetesService).
		WithFile("/root/.kube/config", kubeconfig).
		WithExec([]string{"kubectl", "config", "set-cluster", "minikube", fmt.Sprintf("--server=https://kubernetes:%d", kubernetesPort)}).
		WithExec(inSh("kubectl create namespace %s --dry-run=client --output=json | kubectl apply -f -", namespace))

	c = createSecrets(c, hp.Parent.Main.Creds, name, namespace)

	c = c.WithExec([]string{"sh", "-c", fmt.Sprintf("helm upgrade %s ./package.tgz --atomic --install --namespace %s --wait %s", name, namespace, additionalArgs)}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("helm uninstall %s --namespace %s --wait", name, namespace)}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("kubectl delete namespace %s", namespace)})

	return c
}

type HelmTemplated struct {
	Container *Container
	Parent    *HelmPackage
}

func (ht *HelmTemplated) Check(
	ctx context.Context,
	// Kubernetes version to check against
	// +default="1.29"
	kubernetesVersion string,
) (*HelmTemplated, error) {
	c := ht.Container.
		WithExec(inSh("/root/go/bin/kubectl-validate %s --version %s", templatedDir, kubernetesVersion)).
		WithExec(inSh("pluto detect-files --target-versions k8s=v%s --v 7 --directory %s", kubernetesVersion, templatedDir))

	return &HelmTemplated{Container: c, Parent: ht.Parent}, nil
}

func createSecrets(container *Container, creds []*Cred, name string, namespace string) *Container {
	c := container
	for _, cred := range creds {
		c = c.
			WithSecretVariable("__PASSWORD", cred.UserSecret).
			WithEnvVariable("__USERNAME", cred.UserId).
			WithExec(inSh(`kubectl create secret docker-registry %s-image-secret \
				--namespace=%s \
				--docker-server=ghcr.io \
				--docker-username=$__USERNAME \
				--docker-password=$__PASSWORD \
				--dry-run=client --output=json | \
			kubectl apply -f -`, name, namespace)).
			WithoutSecretVariable("__PASSWORD").
			WithoutEnvVariable("__USERNAME")
	}
	return c
}

func (m *Helm) base(
	ctx context.Context,
) (*Container, error) {
	c := dag.Container().
		// TODO: Actually implement function to update the version
		// @version policy=^3.0.0 resolved=3.19.1
		From("docker.io/library/alpine@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b").
		WithExec([]string{"sh", "-c", "echo '@community https://dl-cdn.alpinelinux.org/alpine/edge/community' >> /etc/apk/repositories"}).
		WithExec([]string{"apk", "add", "go@community=1.22.3-r0"}).
		WithExec([]string{"apk", "add", "git=2.43.4-r0", "kubectl@community=1.30.0-r1", "helm=3.14.2-r2", "npm=10.2.5-r0", "yq=4.35.2-r4"}).
		WithExec([]string{"go", "install", "github.com/norwoodj/helm-docs/cmd/helm-docs@latest"}).
		WithExec([]string{"npm", "install", "-g", "@socialgouv/helm-schema"}).
		WithExec([]string{"helm", "plugin", "install", "https://github.com/helm-unittest/helm-unittest.git"}).
		WithExec(inSh("go install sigs.k8s.io/kubectl-validate@v0.0.4")).
		WithExec(inSh("wget https://github.com/FairwindsOps/pluto/releases/download/v5.19.4/pluto_5.19.4_linux_amd64.tar.gz -O pluto.tgz && tar -zxvf pluto.tgz pluto && mv pluto /usr/bin/pluto && rm pluto.tgz"))

	if len(m.Main.AdditionalCAs) > 0 {
		for _, ca := range m.Main.AdditionalCAs {
			name, error := ca.Name(ctx)
			if error != nil {
				return nil, error
			}
			c = c.
				WithWorkdir("/usr/local/share/ca-certificates/").
				WithMountedSecret(name, ca)
		}
		c = c.WithExec([]string{"update-ca-certificates"})
	}

	for _, cred := range m.Main.Creds {
		c = c.
			WithSecretVariable("GH_TOKEN", cred.UserSecret).
			WithExec([]string{"sh", "-c", fmt.Sprintf("echo $GH_TOKEN | helm registry login --username %s --password-stdin ghcr.io", cred.UserId)}).
			WithoutSecretVariable("GH_TOKEN")
	}

	return c, nil
}
