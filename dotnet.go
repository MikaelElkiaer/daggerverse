package main

import (
	"context"
	"fmt"
)

type Dotnet struct {
	// +private
	Container *Container
	// +private
	Main *MikaelElkiaer
}

// .NET submodule
func (m *MikaelElkiaer) Dotnet(
	ctx context.Context,
) *Dotnet {
	c := dag.Container().
		From("mcr.microsoft.com/dotnet/sdk:8.0-alpine").
		WithExec(inSh("apk add bash"))

	return &Dotnet{Container: c, Main: m}
}

// Build a .NET project
func (m *Dotnet) Build(
	ctx context.Context,
	// Directory containing the source code
	source *Directory,
	// Build configuration to use
	// +default="Release"
	configuration string,
	// Pattern to match the csproj files
	// +default="**/*.csproj"
	csproj string,
	// Pattern to match the sln files
	// +default="*.sln"
	sln string,
) *DotnetBuild {
	c := m.Container.WithWorkdir("/src").
		WithDirectory(".", source, ContainerWithDirectoryOpts{Include: []string{csproj}}).
		WithDirectory(".", source, ContainerWithDirectoryOpts{Include: []string{sln}})

	c = c.WithExec(inSh("dotnet restore --configfile /root/nuget/nuget.config")).
		WithDirectory(".", source, ContainerWithDirectoryOpts{Exclude: []string{"[Dd]ebug/", "[Rr]elease/"}}).
		WithExec(inSh("dotnet build --configuration %s", configuration))

	return &DotnetBuild{Container: c, Configuration: configuration}
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
	userSecret *Secret,
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
		cred, error = getCred(m.Main.Creds, fromCred)
		if error != nil {
			return nil, fmt.Errorf("cred %s not found", fromCred)
		}
	} else {
		for _, c := range m.Main.Creds {
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
	userSecret *Secret,
) (*Dotnet, error) {
	c := m.Container.
		WithExec(inSh("dotnet new nugetconfig --output /root/nuget/")).
		WithSecretVariable("__PASSWORD", userSecret).
		WithEnvVariable("__USERNAME", userId).
		WithExec(inSh("dotnet nuget add source --username $__USERNAME --password $__PASSWORD --store-password-in-clear-text --name %s %s --configfile /root/nuget/nuget.config", name, feed)).
		WithoutSecretVariable("__PASSWORD").
		WithoutEnvVariable("__USERNAME")

	return &Dotnet{Container: c, Main: m.Main}, nil
}

type DotnetBuild struct {
	Container *Container
	//+private
	Configuration string
}

// Run all available tests
func (m *DotnetBuild) Test(
	ctx context.Context,
) *DotnetBuild {
	c := m.Container.
		WithExec(inSh("dotnet test"))

	return &DotnetBuild{Container: c}
}

// Publish with runtime
func (m *DotnetBuild) Publish(
	ctx context.Context,
) *Container {
	c := m.Container.
		WithExec(inSh("dotnet publish --configuration %s --output /app /p:UseAppHost=false --no-restore", m.Configuration))

	runtime := dag.Container().
		From("mcr.microsoft.com/dotnet/runtime:8.0-alpine").
		WithDirectory("/app", c.Directory("/app"))

	return runtime
}
