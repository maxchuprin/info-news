package main

import (
	"encoding/json"
	"info-news/pkg/api"
	"info-news/pkg/rss"
	"info-news/pkg/storage"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// конфигурация приложения
type config struct {
	URLS   []string `json:"rss"`
	Period int      `json:"request_period"`
}

func main() {

	db, err := storage.InitDB()
	if err != nil {
		log.Fatal(err)
	}
	api := api.New(db)

	// чтение и раскодирование файла конфигурации
	b, err := ioutil.ReadFile("cmd/gonews/config.json")
	if err != nil {
		log.Fatal(err)
	}
	var config config
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatal(err)
	}

	// запуск парсинга новостей в отдельном потоке
	// для каждой ссылки
	chPosts := make(chan []storage.Post)
	chErrs := make(chan error)
	for _, url := range config.URLS {
		go parseURL(url, db, chPosts, chErrs, config.Period)
	}

	// запись потока новостей в БД
	go func() {
		for posts := range chPosts {
			db.StoreNews(posts)
		}
	}()

	// обработка потока ошибок
	go func() {
		for err := range chErrs {
			log.Println("ошибка:", err)
		}
	}()

	// запуск веб-сервера с API и приложением
	err = http.ListenAndServe(":8080", api.Router())
	if err != nil {
		log.Fatal(err)
	}
}

// Асинхронное чтение потока RSS. Раскодированные
// новости и ошибки пишутся в каналы.
func parseURL(url string, db *storage.DB, posts chan<- []storage.Post, errs chan<- error, period int) {
	for {
		news, err := rss.Parse(url)
		if err != nil {
			errs <- err
			continue
		}
		posts <- news
		time.Sleep(time.Minute * time.Duration(period))
	}
}
