package main

// 自己更新 (kaiHEN Installer と同じ方式)。
// GitHub リリース (yyoossk/vita-gallery) の最新タグをビルドスタンプと
// 比べて、新しければ画面に知らせる。適用はプラットフォーム別:
//   - Windows: リリースの zip を落として exe を差し替え、再起動する。
//     差し替えた exe は Vita Gallery.exe.old として残り、次の通常起動で消える。
//   - Android: リリースの APK を外部フォルダに落とし、画面がシェルの
//     ブリッジ (VitaBridge) 経由でシステムのインストール画面を開く。
// リリースの解決は API を使わず、リリースページのリダイレクトと
// expanded_assets フラグメントから行う (クォータ不要)。

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

const updateRepo = "yyoossk/vita-gallery"

var stampRe = regexp.MustCompile(`^\d{8}-\d{4}$`)

// ---------- HTTP (Android 用 DNS 対応つき) ----------

// dnsServers は VITA_DNS (Android シェルが渡す) の DNS サーバー一覧。
// Android には /etc/resolv.conf が無く、Go のリゾルバがそのままでは
// 名前解決できないため、シェルが端末の DNS を渡す。PC では未設定。
var dnsServers = func() []string {
	s := os.Getenv("VITA_DNS")
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}()

var dnsRotate uint32

var webTransport = func() *http.Transport {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}
	if len(dnsServers) > 0 {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				i := atomic.AddUint32(&dnsRotate, 1)
				d := net.Dialer{Timeout: 6 * time.Second}
				return d.DialContext(ctx, "udp", net.JoinHostPort(dnsServers[int(i)%len(dnsServers)], "53"))
			},
		}
		tr.DialContext = (&net.Dialer{Timeout: 30 * time.Second, Resolver: resolver}).DialContext
	} else {
		tr.DialContext = (&net.Dialer{Timeout: 30 * time.Second}).DialContext
	}
	return tr
}()

func webClient() *http.Client {
	return &http.Client{Timeout: 300 * time.Second, Transport: webTransport}
}

func httpGet(u string) ([]byte, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VitaGallery")
	resp, err := webClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<20))
}

func httpGetFinal(u string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "VitaGallery")
	resp, err := webClient().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil {
		return nil, "", err
	}
	return b, resp.Request.URL.String(), nil
}

// ---------- リリース解決 ----------

func latestRelease() (string, map[string]string, error) {
	_, final, err := httpGetFinal("https://github.com/" + updateRepo + "/releases/latest")
	if err != nil {
		return "", nil, err
	}
	i := strings.Index(final, "/releases/tag/")
	if i < 0 {
		return "", nil, errors.New("no release tag")
	}
	tag, err := url.PathUnescape(strings.TrimSuffix(final[i+len("/releases/tag/"):], "/"))
	if err != nil || !stampRe.MatchString(tag) {
		return "", nil, errors.New("no release tag")
	}
	body, err := httpGet("https://github.com/" + updateRepo + "/releases/expanded_assets/" + url.PathEscape(tag))
	if err != nil {
		return "", nil, err
	}
	re := regexp.MustCompile(`href="(/[^"]+/releases/download/[^"]+)"`)
	assets := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(string(body), -1) {
		href := m[1]
		ext := strings.ToLower(filepath.Ext(href))
		if (ext == ".zip" || ext == ".apk") && assets[ext] == "" {
			assets[ext] = "https://github.com" + href
		}
	}
	return tag, assets, nil
}

func updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	tag, _, err := latestRelease()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		fmt.Fprintf(w, `{"available":false,"current":%q}`, buildStamp)
		return
	}
	fmt.Fprintf(w, `{"available":%v,"current":%q,"latest":%q}`, tag > buildStamp, buildStamp, tag)
}

// ---------- 適用 (SSE で進捗と結果を返す) ----------

// updateApplyHandler は kaiHEN と同じ SSE 枠組みで進捗を流す:
// 通常行はそのまま、APK 準備完了は "UPDATEAPK|ファイル名"、
// 最後に "RESULT|ok" / "RESULT|ng" (失敗時は直前に "ERR|コード")。
func updateApplyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fl, _ := w.(http.Flusher)
	send := func(s string) {
		for _, line := range strings.Split(s, "\n") {
			fmt.Fprintf(w, "data: %s\n\n", line)
		}
		if fl != nil {
			fl.Flush()
		}
	}
	tag, assets, err := latestRelease()
	if err != nil || tag <= buildStamp {
		send("ERR|none")
		send("RESULT|ng")
		return
	}
	var aerr error
	switch runtime.GOOS {
	case "windows":
		aerr = applyUpdateWindows(tag, assets[".zip"], send)
	case "android":
		aerr = applyUpdateAndroid(tag, assets[".apk"], send)
	default:
		aerr = errors.New("unsupported")
	}
	if aerr != nil {
		send("ERR|" + aerr.Error())
		send("RESULT|ng")
		return
	}
	send("RESULT|ok")
}

func stageZip(zipURL, dir string) error {
	raw, err := httpGet(zipURL)
	if err != nil {
		return errors.New("dl")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return errors.New("zip")
	}
	os.RemoveAll(dir)
	for _, f := range zr.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		dst := filepath.Join(dir, name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(dst, 0o755)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return errors.New("zip")
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return errors.New("zip")
		}
		os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return errors.New("write")
		}
	}
	return nil
}

func applyUpdateWindows(tag, zipURL string, send func(string)) error {
	if zipURL == "" {
		return errors.New("noasset")
	}
	send("LOG|" + zipURL)
	stage := pathOf(".update")
	if err := stageZip(zipURL, stage); err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	newExe := filepath.Join(stage, "Vita Gallery.exe")
	if _, err := os.Stat(newExe); err != nil {
		return errors.New("noasset")
	}
	exePath, err := os.Executable()
	if err != nil {
		return errors.New("write")
	}
	oldPath := exePath + ".old"
	os.Remove(oldPath)
	if err := os.Rename(exePath, oldPath); err != nil {
		return errors.New("write")
	}
	data, err := os.ReadFile(newExe)
	if err != nil {
		os.Rename(oldPath, exePath)
		return errors.New("write")
	}
	if err := os.WriteFile(exePath, data, 0o755); err != nil {
		os.Rename(oldPath, exePath)
		return errors.New("write")
	}
	cmd := exec.Command(exePath)
	cmd.Env = append(os.Environ(), "VITA_UPDATED=1")
	if err := cmd.Start(); err != nil {
		return errors.New("write")
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func applyUpdateAndroid(tag, apkURL string, send func(string)) error {
	if apkURL == "" {
		return errors.New("noasset")
	}
	dir := os.Getenv("VITA_EXTERNAL_DIR")
	if dir == "" {
		return errors.New("unsupported")
	}
	send("LOG|" + apkURL)
	raw, err := httpGet(apkURL)
	if err != nil {
		return errors.New("dl")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return errors.New("zip")
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			found = true
			break
		}
	}
	if !found {
		return errors.New("zip")
	}
	os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, "vita-update.apk")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return errors.New("write")
	}
	send("UPDATEAPK|vita-update.apk")
	return nil
}

// cleanupUpdateLeftovers は更新で残った .old と .update を消す。
// 更新直後の初回起動 (VITA_UPDATED つき) では残す (手で戻せるように)。
func cleanupUpdateLeftovers() {
	if os.Getenv("VITA_UPDATED") != "" {
		return
	}
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
	if exeDir != "" {
		os.RemoveAll(pathOf(".update"))
	}
}
