package scraper

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"

	"libri-crawler/internal/fetcher"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const (
	ChitaiGorodSitemapURL   = "https://www.chitai-gorod.ru/sitemap.xml"
	chitaiGorodImageBaseURL = "https://content.img-gorod.ru/"
	chitaiGorodJSONLDXPath  = "//script[@type='application/ld+json']"
	chitaiGorodNuxtXPath    = "//script[@id='__NUXT_DATA__']"

	chitaiGorodSpineMaxRatio = 0.3
	chitaiGorodImageSize     = "?width=600"
)

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type sitemapURLSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

func (s *Scraper) ChitaiGorodSitemapHandler(ctx context.Context, data []byte) ([]Task, []ScrapedBook, error) {
	var index sitemapIndex
	if err := xml.Unmarshal(data, &index); err != nil {
		return nil, nil, fmt.Errorf("failed to decode sitemap index: %w", err)
	}

	var tasks []Task
	for _, sm := range index.Sitemaps {
		if strings.Contains(sm.Loc, "/sitemap/products") {
			tasks = append(tasks, Task{
				URL:        sm.Loc,
				Type:       TypeDiscovery,
				RawHandler: s.ChitaiGorodProductSitemapHandler,
			})
		}
	}
	if len(tasks) == 0 {
		return nil, nil, fmt.Errorf("no product sitemaps found in sitemap index")
	}
	return tasks, nil, nil
}

func (s *Scraper) ChitaiGorodProductSitemapHandler(ctx context.Context, data []byte) ([]Task, []ScrapedBook, error) {
	var set sitemapURLSet
	if err := xml.Unmarshal(data, &set); err != nil {
		return nil, nil, fmt.Errorf("failed to decode product sitemap: %w", err)
	}

	tasks := make([]Task, 0, len(set.URLs))
	for _, u := range set.URLs {
		tasks = append(tasks, Task{
			URL:     u.Loc,
			Type:    TypeBook,
			Handler: s.ChitaiGorodBookHandler,
		})
	}
	return tasks, nil, nil
}

func (s *Scraper) ChitaiGorodBookHandler(ctx context.Context, node *html.Node) ([]Task, []ScrapedBook, error) {
	ld, ok := chitaiGorodJSONLDBook(node)
	if !ok {
		return nil, nil, nil
	}

	var mainISBN string
	var barcodes []Barcode
	for raw := range strings.SplitSeq(ld.ISBN, ",") {
		isbn := processISBN(raw)
		if isbn == "" {
			continue
		}
		if mainISBN == "" {
			mainISBN = isbn
		}
		barcodes = append(barcodes, Barcode{Type: "isbn", Value: isbn})
	}
	title := strings.TrimSpace(ld.Name)
	if mainISBN == "" || title == "" {
		return nil, nil, nil
	}

	authors := []string{}
	for _, a := range ld.authors() {
		if name := strings.TrimSpace(a); name != "" {
			authors = append(authors, name)
		}
	}

	return nil, []ScrapedBook{{
		ISBN:       mainISBN,
		Title:      title,
		URL:        ld.URL,
		Authors:    authors,
		SourceName: SourceChitaiGorod,
		Images:     s.chitaiGorodImages(ctx, node, ld.imageURL()),
		Barcodes:   barcodes,
	}}, nil
}

func (s *Scraper) chitaiGorodImages(ctx context.Context, node *html.Node, front string) map[ImageSide]string {
	images := map[ImageSide]string{}
	if front == "" {
		return images
	}
	images[SideFront] = front + chitaiGorodImageSize

	// Flat scans come as [front, back, spine]; a narrow 2nd image means the set is present, so the 1st is the back.
	// Without a spine the 1st image is often a table of contents or a 3D mockup, so nothing is taken.
	additional := chitaiGorodNuxtAdditionalImages(node)
	if len(additional) < 2 {
		return images
	}
	back := chitaiGorodImageBaseURL + strings.TrimPrefix(additional[0], "/")
	spine := chitaiGorodImageBaseURL + strings.TrimPrefix(additional[1], "/")
	if cfg := s.probeImage(ctx, spine); cfg != nil && cfg.Height > 0 && float64(cfg.Width)/float64(cfg.Height) < chitaiGorodSpineMaxRatio {
		images[SideBack] = back + chitaiGorodImageSize
		images[SideSpine] = spine + chitaiGorodImageSize
	}
	return images
}

// probeImage reads only the image header to get its dimensions.
func (s *Scraper) probeImage(ctx context.Context, url string) *image.Config {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", fetcher.UserAgent)
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	cfg, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return nil
	}
	return &cfg
}

type chitaiGorodLDBook struct {
	Type   any             `json:"@type"`
	ISBN   string          `json:"isbn"`
	Name   string          `json:"name"`
	URL    string          `json:"url"`
	Author json.RawMessage `json:"author"`
	Image  json.RawMessage `json:"image"`
}

type chitaiGorodLDThing struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// oneOrMany decodes a schema.org value that may be a single object or an array of them.
func oneOrMany(raw json.RawMessage) []chitaiGorodLDThing {
	var many []chitaiGorodLDThing
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	var one chitaiGorodLDThing
	if json.Unmarshal(raw, &one) == nil {
		return []chitaiGorodLDThing{one}
	}
	return nil
}

func (b chitaiGorodLDBook) authors() []string {
	var names []string
	for _, p := range oneOrMany(b.Author) {
		names = append(names, p.Name)
	}
	return names
}

// imageURL returns the cover URL; books without a cover point at a relative site placeholder, which is ignored.
func (b chitaiGorodLDBook) imageURL() string {
	var url string
	if json.Unmarshal(b.Image, &url) != nil {
		for _, img := range oneOrMany(b.Image) {
			if img.URL != "" {
				url = img.URL
				break
			}
		}
	}
	if !strings.HasPrefix(url, "https://") {
		return ""
	}
	return url
}

func chitaiGorodJSONLDBook(node *html.Node) (chitaiGorodLDBook, bool) {
	scripts, _ := htmlquery.QueryAll(node, chitaiGorodJSONLDXPath)
	for _, sc := range scripts {
		var doc struct {
			Graph []json.RawMessage `json:"@graph"`
		}
		if json.Unmarshal([]byte(htmlquery.InnerText(sc)), &doc) != nil {
			continue
		}
		for _, raw := range doc.Graph {
			var b chitaiGorodLDBook
			if json.Unmarshal(raw, &b) == nil && b.Type == "Book" {
				return b, true
			}
		}
	}
	return chitaiGorodLDBook{}, false
}

// chitaiGorodNuxtAdditionalImages returns gallery images after the front from the Nuxt payload,
// a flat JSON array where values reference other entries by index.
// Two product layouts exist: mainImage+additionalImages (new) and picture+images (old).
func chitaiGorodNuxtAdditionalImages(node *html.Node) []string {
	script, _ := htmlquery.Query(node, chitaiGorodNuxtXPath)
	if script == nil {
		return nil
	}
	var arr []json.RawMessage
	if json.Unmarshal([]byte(htmlquery.InnerText(script)), &arr) != nil {
		return nil
	}

	deref := func(ref json.RawMessage, out any) bool {
		var i int
		if json.Unmarshal(ref, &i) != nil || i < 0 || i >= len(arr) {
			return false
		}
		return json.Unmarshal(arr[i], out) == nil
	}
	derefList := func(ref json.RawMessage) []string {
		var refs []json.RawMessage
		if !deref(ref, &refs) {
			return nil
		}
		var paths []string
		for _, r := range refs {
			var p string
			if deref(r, &p) && p != "" {
				paths = append(paths, p)
			}
		}
		return paths
	}

	for _, raw := range arr {
		var obj map[string]json.RawMessage
		if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &obj) != nil {
			continue
		}
		if _, ok := obj["mainImage"]; ok {
			return derefList(obj["additionalImages"])
		}
		if _, ok := obj["picture"]; ok {
			if _, ok := obj["images"]; ok {
				return derefList(obj["images"])
			}
		}
	}
	return nil
}
