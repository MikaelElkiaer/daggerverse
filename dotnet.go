package main

import (
	"context"
)

type Dotnet struct {
	// +private
	Main *MikaelElkiaer
	// +private
	NugetConfig *File
}

// .NET submodule
func (m *MikaelElkiaer) Dotnet(
	ctx context.Context,
) *Dotnet {
	return &Dotnet{Main: m}
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
	c := dag.Container().
		From("mcr.microsoft.com/dotnet/sdk:8.0-alpine").
		WithExec(inSh("apk add bash")).
		WithWorkdir("/src").
		WithDirectory(".", source, ContainerWithDirectoryOpts{Include: []string{csproj}}).
		WithDirectory(".", source, ContainerWithDirectoryOpts{Include: []string{sln}})

	if m.NugetConfig != nil {
		c = c.WithMountedFile("./NuGet.Config", m.NugetConfig)
	}

	c = c.WithExec([]string{"dotnet", "restore"}).
		WithDirectory(".", source, ContainerWithDirectoryOpts{Exclude: []string{"[Dd]ebug/", "[Rr]elease/"}}).
		WithExec(inSh("dotnet build --configuration %s", configuration))

	return &DotnetBuild{Container: c, Configuration: configuration}
}

// Set up NuGet config
func (m *Dotnet) WithNuget(
	ctx context.Context,
	// Path to an existing NuGet.Config file
	// If not provided, a default one will be created
	// +optional
	path *File,
	// The organisation to use for the NuGet source
	// Required if path is not provided
	// +optional
	organisation string,
) *Dotnet {
	var file *File
	if path != nil {
		file = path
	} else if organisation != "" {
		file = m.createNugetConfig(ctx, organisation)
	}

	return &Dotnet{Main: m.Main, NugetConfig: file}
}

func (m *Dotnet) createNugetConfig(
	ctx context.Context,
	organisation string,
) *File {
	return dag.Container().
		From("mcr.microsoft.com/dotnet/sdk:8.0-alpine").
		WithSecretVariable("GH_TOKEN", m.Main.GithubToken).
		WithEnvVariable("GH_USERNAME", m.Main.GithubUsername).
		WithExec(inSh("echo '<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<configuration></configuration>' > /NuGet.Config")).
		WithExec(inSh("cat", "/NuGet.Config")).
		WithExec(inSh("dotnet nuget add source --username $GH_USERNAME --password $GH_TOKEN --store-password-in-clear-text --name %s https://nuget.pkg.github.com/%s/index.json --configfile /NuGet.Config", organisation, organisation)).
		File("/NuGet.Config")
}

type DotnetBuild struct {
	Container     *Container
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
