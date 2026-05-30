package scraper

import (
	"slices"
	"strings"
)

type SourceName string

const (
	SourceKnigaLv   SourceName = "kniga.lv"
	SourceMnogoknig SourceName = "mnogoknig.com"
	SourceAzon      SourceName = "azon.market"
)

var AllSources = []SourceName{
	SourceKnigaLv,
	SourceMnogoknig,
	SourceAzon,
}

func GetSources() []string {
	var names []string
	for _, s := range AllSources {
		names = append(names, string(s))
	}

	return names
}

func GetSourcesString() string {
	return strings.Join(GetSources(), ", ")
}

func IsValidSource(source SourceName) bool {
	return slices.Contains(AllSources, source)
}
