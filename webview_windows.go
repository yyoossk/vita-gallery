//go:build windows

package main

import (
	"github.com/jchv/go-webview2"
)

// openEmbeddedUI は WebView2 ウィンドウで画面を表示する。
// WebView2 が使えない環境では false を返し、既定ブラウザに切り替える。
func openEmbeddedUI(url string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  pathOf(".vitagallery-webview2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Vita Gallery (build " + buildStamp + ")",
			Width:  1020,
			Height: 840,
			Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return true
}
