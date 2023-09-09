package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/madxmike/blog/hotreload"
)

func main() {
	fsRoot := "www"
	fs := os.DirFS(fsRoot)

	templates, err := template.ParseFS(fs, "*.html", "**/*.html")
	if err != nil {
		panic(err)
	}

	hotReloadService, err := hotreload.NewService(fsRoot, fs, templates)
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/hotreload", hotreload.Handler(hotReloadService))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		err := templates.ExecuteTemplate(w, "index.html", nil)
		if err != nil {
			panic(err)
		}
	})

	log.Fatal(http.ListenAndServe(":8080", r))
}
