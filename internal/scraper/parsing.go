package scraper

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func getMetaContent(n *html.Node, itemprop string) string {
	return getAttr(n, fmt.Sprintf("meta[@itemprop='%s']", itemprop), "content")
}

func getAttr(n *html.Node, selector, attr string) string {
	node, _ := htmlquery.Query(n, fmt.Sprintf(".//%s", selector))
	if node == nil {
		return ""
	}
	return htmlquery.SelectAttr(node, attr)
}

var isbnRegex = regexp.MustCompile(`^[\d-]+$`)

func processISBN(isbn string) string {
	isbn = strings.TrimSpace(isbn)
	if !isbnRegex.MatchString(isbn) {
		return ""
	}
	return strings.ReplaceAll(isbn, "-", "")
}
