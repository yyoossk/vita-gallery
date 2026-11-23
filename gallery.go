package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	_ "golang.org/x/image/bmp"
)

const thumbMax = 480
const fileMax = 64 << 20

func validIP(s string) bool { return net.ParseIP(strings.TrimSpace(s)) != nil }

func parsePort(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 || n > 65535 {
		return 1337
	}
	return n
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// エラーはコードで返し、画面側が表示言語に訳す。
const (
	errCodeConnect  = "connect"
	errCodeNoFolder = "nofolder"
	errCodeBadIP    = "badip"
	errCodeList     = "list"
	errCodeBadFile  = "badfile"
	errCodeNoSel    = "nosel"
)

func connectFor(w http.ResponseWriter, r *http.Request) (*vitaSession, bool) {
	q := r.URL.Query()
	ip := q.Get("ip")
	if !validIP(ip) {
		writeErr(w, http.StatusBadRequest, errCodeBadIP)
		return nil, false
	}
	s, err := vitaConnect(ip, parsePort(q.Get("port")))
	if err != nil {
		if strings.Contains(err.Error(), "no SCREENSHOT") {
			writeErr(w, http.StatusBadGateway, errCodeNoFolder)
		} else {
			writeErr(w, http.StatusBadGateway, errCodeConnect)
		}
		return nil, false
	}
	return s, true
}

// ---------- ダウンロード完了の追跡 ----------
// ダウンロードはリンク経由でブラウザ / DownloadManager が行うため、
// 画面は tok を付けて開始し、サーバー側の転送完了をポーリングで知る。

var (
	dlMu     sync.Mutex
	dlStates = map[string]string{} // tok -> run | done | err
)

func dlSet(tok, state string) {
	if tok == "" {
		return
	}
	dlMu.Lock()
	dlStates[tok] = state
	if len(dlStates) > 64 {
		for k := range dlStates {
			if k != tok {
				delete(dlStates, k)
				break
			}
		}
	}
	dlMu.Unlock()
}

func dlStatusHandler(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("tok")
	dlMu.Lock()
	st, ok := dlStates[tok]
	dlMu.Unlock()
	if !ok {
		st = "none"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"state": st})
}

// ---------- 一覧 ----------

type galItem struct {
	Path   string `json:"path"`   // SCREENSHOT からの相対パス (folder/file.jpg)
	Folder string `json:"folder"` // フォルダ名 (直下は "")
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Ext    string `json:"ext"` // jpg | bmp
}

func mediaExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		return "jpg"
	case ".bmp":
		return "bmp"
	}
	return ""
}

// walk は SCREENSHOT 直下のファイルと、各フォルダの中のファイルを集める。
func (s *vitaSession) walk() ([]galItem, error) {
	var items []galItem
	var rec func(abs, rel string, depth int) error
	rec = func(abs, rel string, depth int) error {
		ents, err := s.list(abs)
		if err != nil {
			return err
		}
		for _, e := range ents {
			childRel := e.Name
			if rel != "" {
				childRel = rel + "/" + e.Name
			}
			if e.Dir {
				if depth < 2 {
					if err := rec(abs+"/"+e.Name, childRel, depth+1); err != nil {
						return err
					}
				}
				continue
			}
			if ext := mediaExt(e.Name); ext != "" {
				folder := ""
				if i := strings.LastIndex(childRel, "/"); i >= 0 {
					folder = childRel[:i]
				}
				items = append(items, galItem{Path: childRel, Folder: folder, Name: e.Name, Size: e.Size, Ext: ext})
			}
		}
		return nil
	}
	if err := rec(s.root, "", 0); err != nil {
		return nil, err
	}
	// フォルダ名順 → ファイル名順 (名前順の並びを維持する)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Folder != items[j].Folder {
			return items[i].Folder < items[j].Folder
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func checkHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := connectFor(w, r)
	if !ok {
		return
	}
	root := s.root
	s.close()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "root": root})
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := connectFor(w, r)
	if !ok {
		return
	}
	defer s.close()
	items, err := s.walk()
	if err != nil {
		writeErr(w, http.StatusBadGateway, errCodeList)
		return
	}
	if items == nil {
		items = []galItem{}
	}
	folders := map[string]bool{}
	for _, it := range items {
		folders[it.Folder] = true
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"items": items, "count": len(items), "folders": len(folders), "root": s.root,
	})
}

// cleanRel は相対パスを検証する (フォルダ外への参照を拒否)。
func cleanRel(rel string) (string, bool) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", false
		}
	}
	c := path.Clean("/" + rel)
	if c == "/" {
		return "", false
	}
	rel = strings.TrimPrefix(c, "/")
	if mediaExt(rel) == "" {
		return "", false
	}
	return rel, true
}

func sessionForPath(w http.ResponseWriter, r *http.Request) (*vitaSession, string, bool) {
	rel, ok := cleanRel(r.URL.Query().Get("path"))
	if !ok {
		writeErr(w, http.StatusBadRequest, errCodeBadFile)
		return nil, "", false
	}
	s, ok2 := connectFor(w, r)
	if !ok2 {
		return nil, "", false
	}
	return s, s.abs(rel), true
}

func contentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".bmp":
		return "image/bmp"
	}
	return "application/octet-stream"
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	s, abs, ok := sessionForPath(w, r)
	if !ok {
		return
	}
	defer s.close()
	name := path.Base(abs)
	isDl := r.URL.Query().Get("dl") == "1"
	tok := r.URL.Query().Get("tok")
	if isDl {
		dlSet(tok, "run")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	}
	w.Header().Set("Content-Type", contentType(name))
	if n, has := s.size(abs); has {
		w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
	}
	err := s.retr(abs, w)
	if isDl {
		if err != nil {
			dlSet(tok, "err")
		} else {
			dlSet(tok, "done")
		}
	}
}

// ---------- サムネイル ----------

var (
	thumbMu    sync.Mutex
	thumbCache = map[string][]byte{}
	thumbOrder []string
)

func makeThumb(s *vitaSession, abs, key string) ([]byte, error) {
	thumbMu.Lock()
	if b, ok := thumbCache[key]; ok {
		thumbMu.Unlock()
		return b, nil
	}
	thumbMu.Unlock()
	var buf bytes.Buffer
	if err := s.retr(abs, &limitWriter{w: &buf, n: fileMax}); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(&buf)
	if err != nil {
		return nil, err
	}
	small := scaleToFit(img, thumbMax)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, small, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	b := out.Bytes()
	thumbMu.Lock()
	thumbCache[key] = b
	thumbOrder = append(thumbOrder, key)
	for len(thumbOrder) > 120 {
		delete(thumbCache, thumbOrder[0])
		thumbOrder = thumbOrder[1:]
	}
	thumbMu.Unlock()
	return b, nil
}

type limitWriter struct {
	w io.Writer
	n int64
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	q := p
	if int64(len(q)) > l.n {
		q = q[:l.n]
	}
	n, err := l.w.Write(q)
	l.n -= int64(n)
	if err != nil {
		return n, err
	}
	return len(p), nil
}

func scaleToFit(img image.Image, max int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return img
	}
	nw, nh := w, h
	if w >= h {
		nw, nh = max, h*max/w
	} else {
		nw, nh = w*max/h, max
	}
	if nh < 1 {
		nh = 1
	}
	if nw < 1 {
		nw = 1
	}
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		fy := (float64(y)+0.5)*float64(h)/float64(nh) - 0.5
		y0 := int(fy)
		if y0 < 0 {
			y0, fy = 0, 0
		}
		y1 := y0 + 1
		if y1 >= h {
			y1 = h - 1
		}
		ty := fy - float64(int(fy))
		if y0 == y1 {
			ty = 0
		}
		for x := 0; x < nw; x++ {
			fx := (float64(x)+0.5)*float64(w)/float64(nw) - 0.5
			x0 := int(fx)
			if x0 < 0 {
				x0, fx = 0, 0
			}
			x1 := x0 + 1
			if x1 >= w {
				x1 = w - 1
			}
			tx := fx - float64(int(fx))
			if x0 == x1 {
				tx = 0
			}
			dst.Set(x, y, bilerp(src, x0, y0, x1, y1, tx, ty))
		}
	}
	return dst
}

func bilerp(src *image.RGBA, x0, y0, x1, y1 int, tx, ty float64) color.RGBA {
	c00 := src.RGBAAt(x0, y0)
	c10 := src.RGBAAt(x1, y0)
	c01 := src.RGBAAt(x0, y1)
	c11 := src.RGBAAt(x1, y1)
	mix := func(a, b, c, d uint8) uint8 {
		top := float64(a)*(1-tx) + float64(b)*tx
		bot := float64(c)*(1-tx) + float64(d)*tx
		return uint8(top*(1-ty) + bot*ty + 0.5)
	}
	return color.RGBA{
		R: mix(c00.R, c10.R, c01.R, c11.R),
		G: mix(c00.G, c10.G, c01.G, c11.G),
		B: mix(c00.B, c10.B, c01.B, c11.B),
		A: 255,
	}
}

func thumbHandler(w http.ResponseWriter, r *http.Request) {
	s, abs, ok := sessionForPath(w, r)
	if !ok {
		return
	}
	defer s.close()
	q := r.URL.Query()
	key := q.Get("ip") + "|" + q.Get("port") + "|" + q.Get("path")
	b, err := makeThumb(s, abs, key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(b)
}

// ---------- 削除 ----------

type pathsBody struct {
	IP    string   `json:"ip"`
	Port  int      `json:"port"`
	Path  string   `json:"path"`
	Paths []string `json:"paths"`
}

func readBody(r *http.Request) pathsBody {
	var b pathsBody
	if r.Body != nil {
		json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&b)
	}
	if b.IP == "" {
		b.IP = r.URL.Query().Get("ip")
	}
	if b.Port == 0 {
		b.Port = parsePort(r.URL.Query().Get("port"))
	}
	return b
}

func deletePaths(w http.ResponseWriter, b pathsBody, rels []string) {
	if !validIP(b.IP) {
		writeErr(w, http.StatusBadRequest, errCodeBadIP)
		return
	}
	s, err := vitaConnect(b.IP, b.Port)
	if err != nil {
		writeErr(w, http.StatusBadGateway, errCodeConnect)
		return
	}
	defer s.close()
	deleted := 0
	var failed []string
	for _, rel := range rels {
		clean, ok := cleanRel(rel)
		if !ok {
			failed = append(failed, rel)
			continue
		}
		if err := s.dele(s.abs(clean)); err != nil {
			failed = append(failed, clean)
		} else {
			deleted++
			thumbMu.Lock()
			for k := range thumbCache {
				if strings.HasSuffix(k, "|"+clean) {
					delete(thumbCache, k)
				}
			}
			thumbMu.Unlock()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"deleted": deleted, "failed": failed})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	deletePaths(w, b, []string{b.Path})
}

func deleteManyHandler(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	deletePaths(w, b, b.Paths)
}

// ---------- 一括ダウンロード (ZIP) ----------

func zipHandler(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	tok := r.URL.Query().Get("tok")
	if r.Method == http.MethodGet {
		// リンク経由のダウンロード (Android WebView の DownloadManager 対応)。
		// パスは繰り返しパラメータ p で渡す。
		q := r.URL.Query()
		b.IP = q.Get("ip")
		b.Port = parsePort(q.Get("port"))
		b.Paths = q["p"]
	}
	if !validIP(b.IP) || len(b.Paths) == 0 {
		writeErr(w, http.StatusBadRequest, errCodeNoSel)
		return
	}
	dlSet(tok, "run")
	s, err := vitaConnect(b.IP, b.Port)
	if err != nil {
		dlSet(tok, "err")
		writeErr(w, http.StatusBadGateway, errCodeConnect)
		return
	}
	defer s.close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"vita-screenshots.zip\"")
	zw := zip.NewWriter(w)
	used := map[string]int{}
	for _, rel := range b.Paths {
		clean, ok := cleanRel(rel)
		if !ok {
			continue
		}
		name := clean
		if n := used[name]; n > 0 {
			ext := path.Ext(name)
			name = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
		}
		used[clean]++
		fw, err := zw.Create(name)
		if err != nil {
			continue
		}
		_ = s.retr(s.abs(clean), fw)
	}
	if err := zw.Close(); err != nil {
		dlSet(tok, "err")
	} else {
		dlSet(tok, "done")
	}
}
