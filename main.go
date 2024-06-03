package main

import "fmt"

type MikaelElkiaer struct {
	// +private
	AdditionalCAs []*Secret
	// +private
	Creds []*Cred
}

type Cred struct {
	Name       string
	Url        string
	UserId     string
	UserSecret *Secret
}

// Add an additional CA certificate
func (m *MikaelElkiaer) WithAdditionalCA(
	// File containing the CA
	file *Secret,
) *MikaelElkiaer {
	m.AdditionalCAs = append(m.AdditionalCAs, file)
	return m
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
	userSecret *Secret,
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

func inSh(
	cmd string,
	a ...any,
) []string {
	return []string{"sh", "-c", fmt.Sprintf(cmd, a...)}
}
