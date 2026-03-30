package scraper

import "strings"

type SourceName string

const (
	SourceKnigaLv   SourceName = "kniga.lv"
	SourceMnogoknig SourceName = "mnogoknig.com"
)

var AllSources = []SourceName{
	SourceKnigaLv,
	SourceMnogoknig,
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
