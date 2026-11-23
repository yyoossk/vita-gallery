package main

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed page.html
var pageHTML string

func pageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(strings.Replace(pageHTML, "__BUILD__", buildStamp, 1)))
}
