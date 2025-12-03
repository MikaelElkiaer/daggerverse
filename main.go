package main

import (
	"context"
	"crypto/sha256"
	"dagger/mikael-elkiaer/internal/dagger"
	"encoding/hex"
	"fmt"
)

type MikaelElkiaer struct {
	// +private
	AdditionalCAs []*dagger.File
	// +private
	Creds []*Cred
}

type Cred struct {
	Name       string
	Url        string
	UserId     string
	UserSecret *dagger.Secret
}

// Add an additional CA certificate
func (m *MikaelElkiaer) WithCA(
	// File containing the CA
	file *dagger.File,
) *MikaelElkiaer {
	m.AdditionalCAs = append(m.AdditionalCAs, file)
	return m
}

func (m *MikaelElkiaer) WithDownloadedCA(
	ctx context.Context,
	uri string,
) (*MikaelElkiaer, error) {
	cert := downloadAsFile(ctx, uri)
	m = m.WithCA(cert)
	return m, nil
}

// Add additional creds
func (m *MikaelElkiaer) WithCred(
	// Used as identifier in configs
	// Defaults to userId
	// GitHub: Used as organisation name if set
	// +optional
	name string,
	// URL to the service
	// +default="ghcr.io"
	url string,
	// User name, email, or similar
	userId string,
	// Password, token, or similar
	userSecret *dagger.Secret,
) (*MikaelElkiaer, error) {
	id := name
	if id == "" {
		id = userId
	}
	cred := &Cred{
		Name:       id,
		Url:        url,
		UserId:     userId,
		UserSecret: userSecret,
	}
	m.Creds = append(m.Creds, cred)
	return m, nil
}

func downloadAsFile(
	ctx context.Context,
	uri string,
) *dagger.File {
	h := sha256.New()
	h.Write([]byte(uri))
	hashed := h.Sum(nil)
	name := hex.EncodeToString(hashed)

	return dag.Container().
		From("docker.io/library/alpine:3.23.0@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375").
		WithWorkdir("/tmp").
		WithExec([]string{"wget", "--output-document", name, uri}).
		File(name)
}

func inSh(
	cmd string,
	a ...any,
) []string {
	return []string{"sh", "-c", fmt.Sprintf(cmd, a...)}
}
