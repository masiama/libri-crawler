package main

import (
	"context"
	"fmt"
	"libri/ent"
	"libri/ent/book"
	"net/http"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type Scraper struct {
	Client *http.Client
	DB     *ent.Client
}

func (s *Scraper) GetNodes(ctx context.Context, page int) ([]*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://kniga.lv/shop?page=%d", page), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		return nil, err
	}

	nodes, err := htmlquery.QueryAll(doc, "//div[@class='app-product-card']")
	if err != nil {
		fmt.Printf("error parsing HTML: %s\n", err)
		return nil, err
	}
	return nodes, nil
}

func (s *Scraper) Scrape(ctx context.Context, page int) ([]localBook, bool, error) {
	nodes, err := s.GetNodes(ctx, page)
	if err != nil {
		return nil, false, err
	}
	if len(nodes) == 0 {
		return nil, true, nil
	}
	var books []localBook
	for _, n := range nodes {
		nodeBooks := processNode(n)
		books = append(books, nodeBooks...)
	}
	return books, false, nil
}

func (s *Scraper) saveBooks(ctx context.Context, books []localBook) error {
	bulk := make([]*ent.BookCreate, len(books))
	for i, b := range books {
		bulk[i] = s.DB.Book.
			Create().
			SetIsbn(b.Isbn).
			SetTitle(b.Title).
			SetAuthors(b.Authors).
			SetURL(b.URL)
	}
	return s.DB.Book.CreateBulk(bulk...).
		OnConflictColumns(book.FieldIsbn).
		UpdateNewValues().
		Exec(ctx)
}
