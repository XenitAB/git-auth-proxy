package auth

import (
	"regexp"
	"strings"
)

type Endpoint struct {
	scheme       string
	host         string
	organization string
	project      string
	repository   string
	regexes      []*regexp.Regexp

	Token      string
	Namespaces []string
	SecretName string
}

func (e *Endpoint) ID() string {
	comps := []string{e.host, e.organization}
	if e.project != "" {
		comps = append(comps, e.project)
	}
	comps = append(comps, e.repository)
	return strings.Join(comps, "-")
}
