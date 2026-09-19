package main

import (
	"time"

	"github.com/gocolly/colly/v2"
)

type Product struct {
	PName      string
	PLink      string
	ImgLink    string
	Price      string
	PartNumber string
	Brand      string
	Waight     string
	Passport   string
}

type Category struct {
	Name     string
	Link     string
	Products map[string]Product
}

func main() {
	c := colly.NewCollector(
		colly.AllowedDomains("mydent24.ru"),
		colly.Async(true),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       3 * time.Second,
		RandomDelay: 1 * time.Second,
		Parallelism: 2,
	})

}
