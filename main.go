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
	Weight      string
	Passport    string
	Description string
}

type Category struct {
	Name     string
	Link     string
	Products map[string]Product
}

func errorHandler(r *colly.Response, err error) {
	fmt.Println("Ошибка запроса:", r.StatusCode, err)
}
func main() {
	var mu sync.Mutex
	catColl := colly.NewCollector(
		colly.AllowedDomains("mydent24.ru"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	// Задаем правила задержек, чтобы сайт не заблокировал скрапер за спам
	catColl.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       3 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	// Глобальное хранилище: Ключ = очищенный URL категории, Значение = указатель на Category
	DATA := make(map[string]*Category)

	// Перехватываем ссылки на подкатегории на главной странице каталога
	catColl.OnHTML("ul.card-catalog__subcategory-list2 a.link-subcategory", func(h *colly.HTMLElement) {

		name := h.Text
		link := h.Request.AbsoluteURL(h.Attr("href"))

		DATA[link] = &Category{
			Name:     name,
			Link:     link,
			Products: make(map[string]Product),
		}
	})

	// --- PASS 2: Второй коллектор для глубокого сбора товаров ---
	// Клонируем 'c', чтобы унаследовать настройки авторизации/куки и правила задержек
	tumbColl := catColl.Clone()

	// Включаем асинхронный режим для параллельного обхода категорий
	tumbColl.Async = true

	// Переопределяем лимит: добавляем 2 параллельных потока (Parallelism)
	tumbColl.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       3 * time.Second,
		RandomDelay: 1 * time.Second,
		Parallelism: 2,
	})

	// Логика извлечения данных с превью каждой карточки товара
	tumbColl.OnHTML("div.products-row div.product-card", func(h *colly.HTMLElement) {
		// Очищаем текущий URL от query-параметров (?PAGEN_1=2),
		// чтобы получить чистый адрес категории для поиска ключа в DATA
		currentURL := h.Request.URL
		baseSupCategryURl := currentURL.Scheme + "://" + currentURL.Host + currentURL.Path

		// Достаем картинку (.product-card-img-container img) и преобразуем в абсолютный URL
		imgLink := h.Request.AbsoluteURL(h.ChildAttr(".product-card-img-container img", "src"))

		// Достаем название товара и удаляем лишние пробелы/переносы
		name := strings.TrimSpace(h.ChildText("h3.product-card-title"))

		// Достаем цену и чистим неразрывные HTML-пробелы (\u00a0 / &nbsp;)
		rawPrice := h.ChildText(".product-card-price .price")
		price := strings.TrimSpace(strings.ReplaceAll(rawPrice, "\u00a0", " "))

		// Достаем ссылку на карточку самого товара
		link := h.Request.AbsoluteURL(h.ChildAttr("a.product-link", "href"))

		product := Product{
			PName:   name,
			PLink:   link,
			Price:   price,
			ImgLink: imgLink,
		}

		// Запрещаем другим горутинам одновременно писать в DATA
		mu.Lock()
		if cat, ok := DATA[baseSupCategryURl]; ok {
			cat.Products[link] = product
		}
		mu.Unlock()
	})

	// Обработка пагинации (кнопка "Вперед")
	tumbColl.OnHTML("div.bx-pagination li.bx-pag-next a", func(h *colly.HTMLElement) {
		// Переходим по ссылке следующей страницы через h.Request, сохраняя контекст запроса
		h.Request.Visit(h.Request.AbsoluteURL(h.Attr("href")))
	})
	detColl := tumbColl.Clone()

	detColl.OnHTML("div.product-element", func(h *colly.HTMLElement) {
		currentURL := h.Request.URL.String()
		partNumber := strings.TrimSpace(h.ChildText("div.product-property.code-cml2_article div.property-val"))
		docNum := strings.TrimSpace(h.ChildText("div.product-property.code-md_reg_nomer div.property-val"))
		brand := strings.TrimSpace(h.ChildText("div.product-property.code-brand div.property-val"))
		weight := strings.TrimSpace(h.ChildText("div.product-property[class~='code-'] div.property-val"))
		descript := strings.TrimSpace(h.ChildText("div.product-description div.block-info-text div.text"))

		mu.Lock()
		for _, cat := range DATA {
			if prod, ok := cat.Products[currentURL]; ok {
				prod.PartNumber = partNumber
				prod.Passport = docNum
				prod.Brand = brand
				prod.Weight = weight
				prod.Description = descript

				cat.Products[currentURL] = prod
			}
		}
		mu.Unlock()
	})
	// Отслеживание сетевых ошибок или ответов с ошибками (404, 500 и т.д.)
	catColl.OnError(errorHandler)
	tumbColl.OnError(errorHandler)
	detColl.OnError(errorHandler)
	// 1. Запускаем Pass 1 (синхронно собираем все категории в DATA)
	catColl.Visit("https://mydent24.ru/catalog/")

	// 2. Запускаем Pass 2 (асинхронно отправляем найденные категории в очередь 'pc')
	for k := range DATA {
		tumbColl.Visit(k)
	}

	tumbColl.Wait()

	for _, cat := range DATA {
		for link := range cat.Products {
			detColl.Visit(link)
		}
	}
	// 3. ОБЯЗАТЕЛЬНО: блокируем завершение main(), пока все асинхронные горутины pc не закончат работу

	detColl.Wait()

	for k, v := range DATA {
		fmt.Printf("Категория: %s (Всего товаров: %d)\n", k, len(v.Products))
	}
}
