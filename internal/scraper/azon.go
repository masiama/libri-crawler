package scraper

import (
	"context"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func (s *Scraper) AzonListingHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	nodes, _ := htmlquery.QueryAll(node, "//a[@class='pname']")
	if len(nodes) == 0 {
		return nil, nil, nil
	}

	var nextTasks []Task
	for _, n := range nodes {
		bookURL := htmlquery.SelectAttr(n, "href")
		if strings.TrimSpace(htmlquery.InnerText(n)) != "" {
			nextTasks = append(nextTasks, Task{
				URL:     bookURL,
				Type:    TypeBook,
				Handler: s.AzonBookHandler,
			})
		}
	}

	nextNode, _ := htmlquery.Query(node, "//link[@rel='next']")
	if nextNode != nil {
		nextTasks = append(nextTasks, Task{
			URL:     htmlquery.SelectAttr(nextNode, "href"),
			Type:    TypeDiscovery,
			Handler: s.AzonListingHandler,
		})
	}

	return nextTasks, nil, nil
}

func (s *Scraper) AzonBookHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	isbn := processISBN(getMetaContent(node, "isbn"))
	if isbn == "" {
		return nil, nil, nil
	}
	titleNode, _ := htmlquery.Query(node, "//h1[@itemprop='name']")
	if titleNode == nil {
		return nil, nil, nil
	}
	title := strings.TrimSpace(htmlquery.InnerText(titleNode))

	image := getAttr(node, "img[@itemprop='image']", "src")
	url := getAttr(node, "meta[@property='og:url']", "content")

	authors := []string{}
	authorsSeq := strings.FieldsFuncSeq(
		getMetaContent(node, "book:author"),
		func(r rune) bool { return r == ';' || r == ',' },
	)
	for author := range authorsSeq {
		name := strings.TrimSpace(author)
		if name != "" {
			authors = append(authors, name)
		}
	}

	barcodes := []Barcode{{Type: "isbn", Value: isbn}}
	sku := getMetaContent(node, "sku")
	mpn := getMetaContent(node, "mpn")
	if sku != "" {
		barcodes = append(barcodes, Barcode{Type: "sku", Value: sku})
	}
	if mpn != "" && mpn != sku {
		barcodes = append(barcodes, Barcode{Type: "mpn", Value: mpn})
	}

	return nil, []ScrapedBook{{
		ISBN:       isbn,
		Title:      title,
		URL:        url,
		Authors:    authors,
		SourceName: SourceAzon,
		ImageURL:   image,
		Barcodes:   barcodes,
	}}, nil
}
