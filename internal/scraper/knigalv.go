package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func (s *Scraper) KnigaListingHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	nodes, _ := htmlquery.QueryAll(node, "//div[@class='app-product-card']")
	if len(nodes) == 0 {
		return nil, nil, nil
	}

	var books []ScrapedBook
	for _, n := range nodes {
		nodeBooks := processNode(n)
		books = append(books, nodeBooks...)
	}

	var nextTasks []Task
	firstNode, _ := htmlquery.Query(node, "//div[@class='app-pagination']/a[1]")
	lastNode, _ := htmlquery.Query(node, "//div[@class='app-pagination']/a[last()-1]")
	if firstNode != nil && htmlquery.SelectAttr(firstNode, "class") == "active" && lastNode != nil {
		if lastPageNum, err := strconv.Atoi(htmlquery.InnerText(lastNode)); err == nil {
			for i := 2; i <= lastPageNum; i++ {
				nextTasks = append(nextTasks, Task{
					URL:     fmt.Sprintf("https://kniga.lv/shop?page=%d", i),
					Type:    TypeDiscovery,
					Handler: s.KnigaListingHandler,
				})
			}
		}
	}

	return nextTasks, books, nil
}

func processNode(n *html.Node) []ScrapedBook {
	productIdArr := strings.Split(getMetaContent(n, "productID"), ":")
	if productIdArr[0] != "isbn" {
		return nil
	}

	image := strings.ReplaceAll(getMetaContent(n, "image"), "width=320", "width=600")
	title := getMetaContent(n, "name")
	url := getMetaContent(n, "url")

	authorNode, _ := htmlquery.Query(n, "//div[@class='product-author']")
	authors := []string{}
	if authorNode != nil {
		for author := range strings.SplitSeq(htmlquery.InnerText(authorNode), ",") {
			name := strings.TrimSpace(author)
			if name != "" {
				authors = append(authors, name)
			}
		}
	}

	var books []ScrapedBook
	for isbn := range strings.SplitSeq(productIdArr[1], ",") {
		isbn = processISBN(isbn)
		if isbn == "" {
			continue
		}
		books = append(books, ScrapedBook{
			ISBN:       isbn,
			Title:      title,
			URL:        url,
			Authors:    authors,
			SourceName: SourceKnigaLv,
			ImageURL:   image,
		})
	}

	return books

}

var isbnRegex = regexp.MustCompile(`^[\d-]+$`)

func processISBN(isbn string) string {
	isbn = strings.TrimSpace(isbn)
	if !isbnRegex.MatchString(isbn) {
		return ""
	}
	return strings.ReplaceAll(isbn, "-", "")
}
