package scraper

import (
	"context"
	"libri-crawler/ent"
	"net/url"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

func (s *Scraper) MnogoknigCategoryHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	categories, _ := htmlquery.QueryAll(node, "//div[@x-show]/ul//a[starts-with(@href,'https://mnogoknig.com/ru/categories')]")

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
	nodes, _ := htmlquery.QueryAll(node, "//div[@id='product-content']/div/a")
	if len(nodes) == 0 {
		return nil, nil, nil
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

	firstNode, _ := htmlquery.Query(node, "//nav[@role='navigation']//span[@href='#']")
	if firstNode != nil && htmlquery.InnerText(firstNode) == "1" {
		lastNode, _ := htmlquery.Query(node, "//nav[@role='navigation']//a[@href][not(self::node()[@rel])][last()]")
		parsedUrl, _ := url.Parse(htmlquery.SelectAttr(lastNode, "href"))
		query := parsedUrl.Query()
		if lastPageNum, err := strconv.Atoi(query.Get("page")); err == nil {
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
	isbn := getMetaContent(node, "sku")
	image := getLinkHref(node, "image")
	title := getMetaContent(node, "name")
	url := getLinkHref(node, "url")

	authorNode, _ := htmlquery.Query(node, "//a[starts-with(@href,'https://mnogoknig.com/ru/author/')]")
	var authors []string
	if authorNode != nil {
		for author := range strings.SplitSeq(htmlquery.InnerText(authorNode), ",") {
			name := strings.TrimSpace(author)
			if name != "" {
				authors = append(authors, name)
			}
		}
	}

	return nil, []ScrapedBook{{
		Book: ent.Book{
			Isbn:           isbn,
			Title:          title,
			URL:            url,
			Authors:        authors,
			SourceName:     "mnogoknig",
			SourcePriority: PriorityMnogoknig,
		},
		ImageURL: image,
	}}, nil
}
