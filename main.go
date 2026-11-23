// Vita Gallery — PS Vita のスクリーンショットを VitaShell の FTP で
// PC に表示・ダウンロード・削除する単体 GUI。
//
// インストール不要。exe を起動するとローカルサーバーが立ち、
// WebView2 ウィンドウ (無ければ既定ブラウザ) に画面が出る。
// 対象は ux0:picture/SCREENSHOT の中 (フォルダ別) の .jpg / .bmp。
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

var buildStamp = "dev"

var exeDir string

func main() {
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	} else {
		exeDir = "."
	}
	cleanupUpdateLeftovers()
	mux := http.NewServeMux()
	mux.HandleFunc("/", pageHandler)
	mux.HandleFunc("/api/update/check", updateCheckHandler)
	mux.HandleFunc("/api/update/apply", updateApplyHandler)
	mux.HandleFunc("/api/check", checkHandler)
	mux.HandleFunc("/api/list", listHandler)
	mux.HandleFunc("/api/file", fileHandler)
	mux.HandleFunc("/api/thumb", thumbHandler)
	mux.HandleFunc("/api/delete", deleteHandler)
	mux.HandleFunc("/api/delete-many", deleteManyHandler)
	mux.HandleFunc("/api/zip", zipHandler)
	mux.HandleFunc("/api/dlstatus", dlStatusHandler)
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
		go func() { time.Sleep(300 * time.Millisecond); os.Exit(0) }()
	})

	var ln net.Listener
	var err error
	for _, p := range []int{47660, 47661, 47662, 47663, 47664} {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			break
		}
	}
	if ln == nil {
		fmt.Fprintln(os.Stderr, "no free port in 47660-47664")
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s/?v=%d", ln.Addr().String(), time.Now().UnixNano())
	fmt.Println("Vita Gallery:", url)
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	if !openEmbeddedUI(url) {
		openBrowser(url)
		select {}
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func pathOf(parts ...string) string {
	return filepath.Join(append([]string{exeDir}, parts...)...)
}
