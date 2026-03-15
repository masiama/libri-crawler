package scraper

import (
	"fmt"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func getMetaContent(n *html.Node, itemprop string) string {
	metaNode, _ := htmlquery.Query(n, fmt.Sprintf("//meta[@itemprop='%s']", itemprop))
	if metaNode == nil {
		return ""
	}
	return htmlquery.SelectAttr(metaNode, "content")
}

func getLinkHref(n *html.Node, itemprop string) string {
	metaNode, _ := htmlquery.Query(n, fmt.Sprintf("//link[@itemprop='%s']", itemprop))
	if metaNode == nil {
		return ""
	}
	return htmlquery.SelectAttr(metaNode, "href")
}
