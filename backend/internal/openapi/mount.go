package openapi

import (
	"embed"
	"net/http"
)

//go:generate python3 generate.py
//go:embed openapi.json swagger.html static/*
var files embed.FS

func Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		data, _ := files.ReadFile("openapi.json")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		data, _ := files.ReadFile("swagger.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.Handle("GET /swagger/static/", http.StripPrefix("/swagger/", http.FileServerFS(files)))
}
