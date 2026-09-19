package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type Product struct {
	PName       string
	PLink       string
	ImgLink     string
	Price       string
	PartNumber  string
	Brand       string
	Waight      string
	Passport    string
	Description string
}

type Category struct {
	Name     string
	Link     string
	Products map[string]Product
}

func main() {
	var mu sync.Mutex
	c := colly.NewCollector(
		colly.AllowedDomains("mydent24.ru"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       3 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	DATA := make(map[string]*Category)

	c.OnHTML("ul.card-catalog__subcategory-list2 a.link-subcategory", func(h *colly.HTMLElement) {

		name := h.Text
		link := h.Request.AbsoluteURL(h.Attr("href"))
		DATA[link] = &Category{
			Name:     name,
			Link:     link,
			Products: make(map[string]Product),
		}
	})
	pc := c.Clone()
	pc.Async = true
	pc.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       3 * time.Second,
		RandomDelay: 1 * time.Second,
		Parallelism: 2,
	})
	pc.OnHTML("div.products-row div.product-card", func(h *colly.HTMLElement) {
		currentURL := h.Request.URL
		baseSupCategryURl := currentURL.Scheme + "://" + currentURL.Host + currentURL.Path
		imgLink := h.Request.AbsoluteURL(h.ChildAttr("product-card-img-container img", "src"))
		name := strings.TrimSpace(h.ChildText("h3.product-card-title"))
		rawPrice := h.ChildText("product-card-price .price")
		price := strings.TrimSpace(strings.ReplaceAll(rawPrice, "\u00a0", " "))
		link := h.Request.AbsoluteURL(h.ChildAttr("a.product-link", "href"))

		product := Product{
			PName:   name,
			PLink:   link,
			Price:   price,
			ImgLink: imgLink,
		}
		mu.Lock()
		if cat, ok := DATA[baseSupCategryURl]; ok {
			cat.Products[link] = product
		}
		mu.Unlock()
	})

	pc.OnHTML("div.bx-pagination li.bx-pag-next a", func(h *colly.HTMLElement) {
		h.Request.Visit(h.Request.AbsoluteURL(h.Attr("href")))
	})

	pc.OnError(func(r *colly.Response, err error) {
		fmt.Println("Ошибка:", r.StatusCode, err)
	})

	c.Visit("https://mydent24.ru/catalog/")
	for k := range DATA {
		pc.Visit(k)
	}
	pc.Wait()
	for k, v := range DATA {
		fmt.Println(k, " :  ", v)
	}
}
