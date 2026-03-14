package main

import (
	"fmt"
	"libri/ent"
	"regexp"
	"strings"

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

var isbnRegex = regexp.MustCompile(`^[\d-]+$`)

func processISBN(isbn string) string {
	isbn = strings.TrimSpace(isbn)
	if !isbnRegex.MatchString(isbn) {
		return ""
	}
	return strings.ReplaceAll(isbn, "-", "")
}

func processNode(n *html.Node) []localBook {
	productIdArr := strings.Split(getMetaContent(n, "productID"), ":")
	if productIdArr[0] != "isbn" {
		return nil
	}

	image := strings.ReplaceAll(getMetaContent(n, "image"), "width=320", "width=600")
	title := getMetaContent(n, "name")
	url := getMetaContent(n, "url")

	authorNode, _ := htmlquery.Query(n, ".//div[@class='product-author']")
	var authors []string
	if authorNode != nil {
		for author := range strings.SplitSeq(htmlquery.InnerText(authorNode), ",") {
			name := strings.TrimSpace(author)
			if name != "" {
				authors = append(authors, name)
			}
		}
	}

	var books []localBook
	for isbn := range strings.SplitSeq(productIdArr[1], ",") {
		isbn = processISBN(isbn)
		if isbn == "" {
			continue
		}
		books = append(books, localBook{
			Book:     ent.Book{Isbn: isbn, Title: title, URL: url, Authors: authors},
			imageURL: image,
		})
	}

	return books

}
