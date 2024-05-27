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
	UserId     string
	UserSecret *Secret
}

// Add an additional CA certificate
func (m *MikaelElkiaer) WithAdditionalCA(
	// Path to a file containing the CA
	path *Secret,
) *MikaelElkiaer {
	m.AdditionalCAs = append(m.AdditionalCAs, path)
	return m
}

// Add additional creds
func (m *MikaelElkiaer) WithCred(
	// Used as identifier in configs
	// Defaults to userId
	// GitHub: Used as organisation name if set
	// +optional
	name string,
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
