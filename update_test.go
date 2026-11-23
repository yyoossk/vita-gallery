package main

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeZipServer(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	}))
}

func TestStageZipExtractsAndRejectsTraversal(t *testing.T) {
	srv := makeZipServer(t, map[string]string{
		"Vita Gallery.exe": "FAKEEXE",
		"../evil.txt":      "EVIL",
		"sub/readme.txt":   "OK",
	})
	defer srv.Close()
	dir := filepath.Join(t.TempDir(), "stage")
	if err := stageZip(srv.URL, dir); err != nil {
		t.Fatalf("stageZip: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "Vita Gallery.exe"))
	if err != nil || string(b) != "FAKEEXE" {
		t.Errorf("exe = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); !os.IsNotExist(err) {
		t.Errorf("traversal entry escaped the stage dir")
	}
}

func TestApplyUpdateAndroid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VITA_EXTERNAL_DIR", dir)
	var sent []string
	send := func(s string) { sent = append(sent, s) }

	// APK でない中身は zip エラー
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not a zip"))
	}))
	defer bad.Close()
	if err := applyUpdateAndroid("99999999-9999", bad.URL, send); err == nil || err.Error() != "zip" {
		t.Errorf("bad apk err = %v, want zip", err)
	}

	// AndroidManifest.xml を含む zip は APK として保存される
	good := makeZipServer(t, map[string]string{"AndroidManifest.xml": "x", "classes.dex": "y"})
	defer good.Close()
	if err := applyUpdateAndroid("99999999-9999", good.URL, send); err != nil {
		t.Fatalf("applyUpdateAndroid: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "vita-update.apk")); err != nil {
		t.Errorf("vita-update.apk not written: %v", err)
	}
	joined := strings.Join(sent, " ")
	if !strings.Contains(joined, "UPDATEAPK|vita-update.apk") {
		t.Errorf("UPDATEAPK marker not sent: %v", sent)
	}

	// アセット無し
	if err := applyUpdateAndroid("99999999-9999", "", send); err == nil || err.Error() != "noasset" {
		t.Errorf("empty url err = %v, want noasset", err)
	}
}

func TestStampOrdering(t *testing.T) {
	if !stampRe.MatchString("20261123-1846") || stampRe.MatchString("v1.0") {
		t.Errorf("stampRe broken")
	}
	if !("20261123-1900" > "20261123-1846") || "20261123-1846" > "20261123-1846" {
		t.Errorf("stamp string ordering broken")
	}
}
