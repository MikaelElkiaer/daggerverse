package main

import (
	"context"
	"dagger/mikael-elkiaer/internal/dagger"
	_ "embed"
	"fmt"
)

//go:embed assets/dotnet.Dockerfile
var dotnet__Dockerfile string

type Dotnet struct {
	// +private
	Base *dagger.Container
	// +private
	Configuration string
	// Latest run container, contains workdir
	Container *dagger.Container
	// +private
	Module *MikaelElkiaer
	// +private
	EntrypointProject string
}

// .NET submodule
func (m *MikaelElkiaer) Dotnet(
	ctx context.Context,
	// Configuration to use for commands
	// +default="Release"
	configuration string,
	// Name of the entrypoint project
	entrypointProject string,
	// Solution directory
	source *dagger.Directory,
) *Dotnet {
	c := dag.Container().
		From("mcr.microsoft.com/dotnet/sdk:8.0-alpine").
		WithExec(inSh(`apk add --no-cache bash`)).
		WithWorkdir("/src").
		WithExec(inSh("dotnet new nugetconfig --output /root/nuget/"))

	return &Dotnet{Base: c, Configuration: configuration, Container: c.WithDirectory(WORKDIR, source), Module: m, EntrypointProject: entrypointProject}
}

// Restore dependencies
func (m *Dotnet) Restore(
	ctx context.Context,
	// Pattern to match the csproj files
	// +default="**/*.csproj"
	csproj string,
	// Pattern to match the sln files
	// +default="*.sln"
	sln string,
) *Dotnet {
	m.Container = m.Base.
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR), dagger.ContainerWithDirectoryOpts{Include: []string{csproj}}).
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR), dagger.ContainerWithDirectoryOpts{Include: []string{sln}}).
		WithExec(inSh("dotnet restore --configfile /root/nuget/nuget.config --packages .packages")).
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR))

	return m
}

// Build a .NET project
func (m *Dotnet) Build(
	ctx context.Context,
) *Dotnet {
	m.Container = m.Base.
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR)).
		WithExec(inSh("dotnet build --configuration %s --no-restore --packages .packages", m.Configuration))

	return m
}

// Run all available tests
func (m *Dotnet) Test(
	ctx context.Context,
) *Dotnet {
	m.Container = m.Base.
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR)).
		WithExec(inSh("dotnet test --configuration %s --no-build", m.Configuration))

	return m
}

// Publish with runtime
func (m *Dotnet) Publish(
	ctx context.Context,
) *Dotnet {
	m.Container = m.Base.
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR)).
		WithWorkdir(m.EntrypointProject).
		WithExec(inSh("dotnet publish --configuration %s --no-build --output ../app /p:UseAppHost=false", m.Configuration))

	return m
}

// Build container with runtime
func (m *Dotnet) BuildContainer(
	ctx context.Context,
	// Tag to use as image name
	// +default=""
	tag string,
) *dagger.Container {
	c := dag.Container().
		WithDirectory(WORKDIR, m.Container.Directory(WORKDIR)).
		WithWorkdir(WORKDIR).
		WithNewFile("Dockerfile", dotnet__Dockerfile).
		Directory("/src").
		DockerBuild(dagger.DirectoryDockerBuildOpts{
			BuildArgs: []dagger.BuildArg{
				{Name: "PROJECT_NAME", Value: m.EntrypointProject},
			}},
		)

	if tag != "" {
		c = c.WithAnnotation("io.containerd.image.name", tag)
	}

	return c
}

// Set up NuGet config
func (m *Dotnet) WithNuget(
	ctx context.Context,
	// NuGet feed URL
	feed string,
	// Used as identifier in configs
	name string,
	// User name, email, or similar
	userId string,
	// Password, token, or similar
	userSecret *dagger.Secret,
) (*Dotnet, error) {
	return m.withNuget(ctx, feed, name, userId, userSecret)
}

// Set up NuGet config for GitHub Container Registry
func (m *Dotnet) WithNugetGhcr(
	ctx context.Context,
	// Credential to use
	// Defaults to the first credential
	// +optional
	fromCred string,
) (*Dotnet, error) {
	var cred *Cred
	if fromCred != "" {
		var error error
		cred, error = getCred(m.Module.Creds, fromCred)
		if error != nil {
			return nil, fmt.Errorf("cred %s not found", fromCred)
		}
	} else {
		for _, c := range m.Module.Creds {
			cred = c
			break
		}
	}

	if cred == nil {
		return nil, fmt.Errorf("no creds found")
	}

	feed := fmt.Sprintf("https://nuget.pkg.github.com/%s/index.json", cred.Name)
	return m.withNuget(ctx, feed, cred.Name, cred.UserId, cred.UserSecret)
}

func getCred(
	creds []*Cred,
	fromCred string,
) (*Cred, error) {
	var cred *Cred
	for _, c := range creds {
		if c.Name == fromCred {
			if cred != nil {
				return nil, fmt.Errorf("multiple creds with name %s found", fromCred)
			}
			cred = c
		}
	}

	if cred == nil {
		return nil, fmt.Errorf("cred %s not found", fromCred)
	}

	return cred, nil
}

func (m *Dotnet) withNuget(
	ctx context.Context,
	feed string,
	name string,
	userId string,
	userSecret *dagger.Secret,
) (*Dotnet, error) {
	m.Base = m.Base.
		WithSecretVariable("__PASSWORD", userSecret).
		WithEnvVariable("__USERNAME", userId).
		WithExec(inSh("dotnet nuget add source --username $__USERNAME --password $__PASSWORD --store-password-in-clear-text --name %s %s --configfile /root/nuget/nuget.config", name, feed)).
		WithoutSecretVariable("__PASSWORD").
		WithoutEnvVariable("__USERNAME")

	return m, nil
}
