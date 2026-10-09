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
	azonProductLinkXPath         = "//a[@class='pname']"
	azonPaginationContainerXPath = "//ul[@class='pagination']"
	azonPaginationFirstXPath     = ".//li[@class='active']"
	azonPaginationLastXPath      = ".//li[@class='last']/a"
)

func (s *Scraper) AzonListingHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	nodes, _ := htmlquery.QueryAll(node, azonProductLinkXPath)
	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("no product links found, selector may be broken: %s", azonProductLinkXPath)
	}

	var nextTasks []Task
	for _, n := range nodes {
		bookURL := htmlquery.SelectAttr(n, "href")
		if strings.TrimSpace(htmlquery.InnerText(n)) != "" {
			bookURL = azonReachableURL(bookURL, getAttr(n, "ancestor::div[@class='product-thumb']//*[@data-product-id]", "data-product-id"))
			nextTasks = append(nextTasks, Task{
				URL:     bookURL,
				Type:    TypeBook,
				Handler: s.AzonBookHandler,
			})
		}
	}

	paginationNode, _ := htmlquery.Query(node, azonPaginationContainerXPath)
	if paginationNode != nil {
		currentNode, _ := htmlquery.Query(paginationNode, azonPaginationFirstXPath)
		if currentNode == nil {
			return nil, nil, fmt.Errorf("pagination present but current-page selector found nothing, selector may be broken: %s", azonPaginationFirstXPath)
		}

		if htmlquery.InnerText(currentNode) == "1" {
			lastNode, _ := htmlquery.Query(paginationNode, azonPaginationLastXPath)
			if lastNode == nil {
				return nil, nil, fmt.Errorf("pagination present but last-page selector found nothing, selector may be broken: %s", azonPaginationLastXPath)
			}

			parsedUrl, err := url.Parse(htmlquery.SelectAttr(lastNode, "href"))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse last page url: %w", err)
			}
			query := parsedUrl.Query()
			query.Set("show_instock", "2")
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
					Handler: s.AzonListingHandler,
				})
			}
		}
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
	if strings.Contains(image, "/placeholder-") {
		image = ""
	}
	url := azonReachableURL(
		getAttr(node, "meta[@property='og:url']", "content"),
		getAttr(node, "input[@name='product_id']", "value"),
	)

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
		Images:     map[ImageSide]string{SideFront: image},
		Barcodes:   barcodes,
	}}, nil
}

func azonReachableURL(rawURL, productID string) string {
	u, err := url.Parse(rawURL)
	if err != nil || productID == "" || !strings.HasPrefix(u.Path, "/blog") {
		return rawURL
	}
	return "https://azon.market/index.php?route=product/product&product_id=" + productID
}
