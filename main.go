package main

type MikaelElkiaer struct {
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
) *MikaelElkiaer {
	return &MikaelElkiaer{
		AdditionalCAs:  additionalCAs,
		GithubToken:    githubToken,
		GithubUsername: githubUsername,
	}
}
