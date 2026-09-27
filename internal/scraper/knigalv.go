package scraper

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const (
	knigaProductCardXPath         = "//div[@class='app-product-card']"
	knigaPaginationContainerXPath = "//div[@class='app-pagination']"
	knigaPaginationFirstXPath     = "./a[1]"
	knigaPaginationLastXPath      = "./a"
	knigaAuthorXPath              = ".//div[@class='product-author']"
)

func (s *Scraper) KnigaListingHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	nodes, _ := htmlquery.QueryAll(node, knigaProductCardXPath)
	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("no product cards found, selector may be broken: %s", knigaProductCardXPath)
	}

	var books []ScrapedBook
	for _, n := range nodes {
		nodeBooks := processNode(n)
		books = append(books, nodeBooks...)
	}

	var nextTasks []Task
	paginationNode, _ := htmlquery.Query(node, knigaPaginationContainerXPath)
	if paginationNode != nil {
		firstNode, _ := htmlquery.Query(paginationNode, knigaPaginationFirstXPath)
		if firstNode == nil {
			return nil, nil, fmt.Errorf("pagination present but first-page-link selector found nothing, selector may be broken: %s", knigaPaginationFirstXPath)
		}

		if htmlquery.SelectAttr(firstNode, "class") == "active" {
			anchors, _ := htmlquery.QueryAll(paginationNode, knigaPaginationLastXPath)
			if len(anchors) < 2 {
				return nil, nil, fmt.Errorf("pagination present but last-page-link selector found fewer than 2 links, selector may be broken: %s", knigaPaginationLastXPath)
			}
			lastNode := anchors[len(anchors)-2]

			lastPageNum, err := strconv.Atoi(htmlquery.InnerText(lastNode))
			if err != nil {
				return nil, nil, fmt.Errorf("last page link text is not a valid page number: %w", err)
			}

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

	authorNode, _ := htmlquery.Query(n, knigaAuthorXPath)
	authors := []string{}
	if authorNode != nil {
		for author := range strings.SplitSeq(htmlquery.InnerText(authorNode), ",") {
			name := strings.TrimSpace(author)
			if name != "" {
				authors = append(authors, name)
			}
		}
	}

	var mainISBN string
	var barcodes []Barcode
	for isbn := range strings.SplitSeq(productIdArr[1], ",") {
		isbn = processISBN(isbn)
		if isbn == "" {
			continue
		}
		if mainISBN == "" {
			mainISBN = isbn
		}
		barcodes = append(barcodes, Barcode{Type: "isbn", Value: isbn})
	}
	if mainISBN == "" {
		return nil
	}

	mpn := getMetaContent(n, "mpn")
	if mpn != "" {
		barcodes = append(barcodes, Barcode{Type: "mpn", Value: mpn})
	}
	sku := getMetaContent(n, "sku")
	if sku != "" {
		barcodes = append(barcodes, Barcode{Type: "sku", Value: sku})
	}

	return []ScrapedBook{{
		ISBN:       mainISBN,
		Title:      title,
		URL:        url,
		Authors:    authors,
		SourceName: SourceKnigaLv,
		ImageURL:   image,
		Barcodes:   barcodes,
	}}
}
