package scraper

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const (
	mnogoknigCategoryLinkXPath        = "//div[@x-show]/ul//a[starts-with(@href,'https://mnogoknig.com/ru/categories')]"
	mnogoknigProductCardXPath         = "//div[contains(concat(' ', @class, ' '), ' product-card ')]//a[@title]"
	mnogoknigPaginationContainerXPath = "//ul[contains(concat(' ', @class, ' '), ' pagination-nav ')]"
	mnogoknigPaginationFirstXPath     = ".//span[@aria-current='page']"
	mnogoknigPaginationLastXPath      = ".//a[@href][not(@rel)][last()]"
	mnogoknigAuthorLinkXPath          = "//a[starts-with(@href,'https://mnogoknig.com/ru/author/')]"
)

func (s *Scraper) MnogoknigCategoryHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	categories, _ := htmlquery.QueryAll(node, mnogoknigCategoryLinkXPath)

	if len(categories) == 0 {
		return s.MnogoknigListingHandler(ctx, node)
	}

	var nextTasks []Task
	for _, category := range categories {
		categoryURL := htmlquery.SelectAttr(category, "href")
		nextTasks = append(nextTasks, Task{
			URL:     categoryURL,
			Type:    TypeDiscovery,
			Handler: s.MnogoknigCategoryHandler,
		})
	}

	return nextTasks, nil, nil
}

func (s *Scraper) MnogoknigListingHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	nodes, _ := htmlquery.QueryAll(node, mnogoknigProductCardXPath)
	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("no product cards found, selector may be broken: %s", mnogoknigProductCardXPath)
	}

	var nextTasks []Task
	for _, n := range nodes {
		bookURL := htmlquery.SelectAttr(n, "href")
		nextTasks = append(nextTasks, Task{
			URL:     bookURL,
			Type:    TypeBook,
			Handler: s.MnogoknigBookHandler,
		})
	}

	paginationNode, _ := htmlquery.Query(node, mnogoknigPaginationContainerXPath)
	if paginationNode != nil {
		currentNode, _ := htmlquery.Query(paginationNode, mnogoknigPaginationFirstXPath)
		if currentNode == nil {
			return nil, nil, fmt.Errorf("pagination present but current-page selector found nothing, selector may be broken: %s", mnogoknigPaginationFirstXPath)
		}

		if htmlquery.InnerText(currentNode) == "1" {
			lastNode, _ := htmlquery.Query(paginationNode, mnogoknigPaginationLastXPath)
			if lastNode == nil {
				return nil, nil, fmt.Errorf("pagination present but last-page selector found nothing, selector may be broken: %s", mnogoknigPaginationLastXPath)
			}

			parsedUrl, err := url.Parse(htmlquery.SelectAttr(lastNode, "href"))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse last page url: %w", err)
			}
			query := parsedUrl.Query()
			lastPageNum, err := strconv.Atoi(query.Get("page"))
			if err != nil {
				return nil, nil, fmt.Errorf("last page link has no valid page query param: %w", err)
			}

			for i := 2; i <= lastPageNum; i++ {
				query.Set("page", strconv.Itoa(i))
				parsedUrl.RawQuery = query.Encode()
				nextTasks = append(nextTasks, Task{
					URL:     parsedUrl.String(),
					Type:    TypeDiscovery,
					Handler: s.MnogoknigListingHandler,
				})
			}
		}
	}

	return nextTasks, nil, nil
}

func (s *Scraper) MnogoknigBookHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	isbn := getMetaContent(node, "gtin")
	image := getAttr(node, "link[@itemprop='image']", "href")
	title := getMetaContent(node, "name")
	url := getAttr(node, "link[@itemprop='url']", "href")
	mpn := getMetaContent(node, "mpn")

	authorNode, _ := htmlquery.Query(node, mnogoknigAuthorLinkXPath)
	authors := []string{}
	if authorNode != nil {
		for author := range strings.SplitSeq(htmlquery.InnerText(authorNode), ",") {
			name := strings.TrimSpace(author)
			if name != "" {
				authors = append(authors, name)
			}
		}
	}

	barcodes := []Barcode{{Type: "isbn", Value: isbn}}
	if mpn != "" {
		barcodes = append(barcodes, Barcode{Type: "mpn", Value: mpn})
	}

	return nil, []ScrapedBook{{
		ISBN:       isbn,
		Title:      title,
		URL:        url,
		Authors:    authors,
		SourceName: SourceMnogoknig,
		ImageURL:   image,
		Barcodes:   barcodes,
	}}, nil
}
