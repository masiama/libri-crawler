package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
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
	azonSubcategoryXPath         = "//li[starts-with(@class,'category_id_')]//a"
	azonResultsInfoXPath         = "//span[@id='cat-results-info']"

	azonMaxPages = 999
)

var azonFoundRegex = regexp.MustCompile(`Найдено\s+(\d+)`)

func (s *Scraper) AzonRootHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	if info, _ := htmlquery.Query(node, azonResultsInfoXPath); info != nil {
		if m := azonFoundRegex.FindStringSubmatch(htmlquery.InnerText(info)); m != nil {
			total, _ := strconv.Atoi(m[1])
			s.EstimatedBooks.Store(int64(total))
		}
	}
	return s.AzonCategoryHandler(ctx, node)
}

func (s *Scraper) AzonCategoryHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	nodes, _ := htmlquery.QueryAll(node, azonSubcategoryXPath)
	own, _, err := s.AzonListingHandler(ctx, node)
	if err != nil {
		return nil, nil, err
	}

	var nextTasks []Task
	for _, n := range nodes {
		nextTasks = append(nextTasks, Task{
			URL:     htmlquery.SelectAttr(n, "href"),
			Type:    TypeDiscovery,
			Handler: s.AzonCategoryHandler,
		})
	}

	return append(nextTasks, own...), nil, nil
}

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
			lastPageNum, err := strconv.Atoi(query.Get("page"))
			if err != nil {
				return nil, nil, fmt.Errorf("last page link has no valid page query param: %w", err)
			}

			lastPageNum = min(lastPageNum, azonMaxPages)
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
	isbn := processISBN(azonMeta(node, "book:isbn"))
	if isbn == "" {
		return nil, nil, nil
	}
	titleNode, _ := htmlquery.Query(node, "//h1[@id='product_title']")
	if titleNode == nil {
		return nil, nil, nil
	}
	title := strings.TrimSpace(htmlquery.InnerText(titleNode))

	image := azonMeta(node, "og:image")
	if strings.Contains(image, "/placeholder-") {
		image = ""
	}
	url := azonReachableURL(
		azonMeta(node, "og:url"),
		getAttr(node, "input[@name='product_id']", "value"),
	)

	authors := []string{}
	authorsSeq := strings.FieldsFuncSeq(
		azonMeta(node, "book:author"),
		func(r rune) bool { return r == ';' || r == ',' },
	)
	for author := range authorsSeq {
		name := strings.TrimSpace(author)
		if name != "" {
			authors = append(authors, name)
		}
	}

	barcodes := []Barcode{{Type: "isbn", Value: isbn}}
	if sku := azonMeta(node, "product:retailer_item_id"); sku != "" {
		barcodes = append(barcodes, Barcode{Type: "sku", Value: sku})
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

func azonMeta(node *html.Node, property string) string {
	return getAttr(node, fmt.Sprintf("meta[@property='%s']", property), "content")
}

func azonReachableURL(rawURL, productID string) string {
	u, err := url.Parse(rawURL)
	if err != nil || productID == "" || !strings.HasPrefix(u.Path, "/blog") {
		return rawURL
	}
	return "https://azon.market/index.php?route=product/product&product_id=" + productID
}
