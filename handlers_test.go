package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// HTTP ハンドラ越しの end-to-end: 一覧→ファイル→サムネ→ZIP→削除
func TestHandlersEndToEnd(t *testing.T) {
	f := newFakeFTP(t)
	q := "ip=127.0.0.1&port=" + strconv.Itoa(f.port)

	// 一覧
	rec := httptest.NewRecorder()
	listHandler(rec, httptest.NewRequest("GET", "/api/list?"+q, nil))
	if rec.Code != 200 {
		t.Fatalf("list code %d body %s", rec.Code, rec.Body.String())
	}
	var lr struct {
		Items []galItem `json:"items"`
		Count int       `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lr); err != nil || lr.Count != 4 {
		t.Fatalf("list body: %v %+v", err, lr)
	}

	// ファイル (bmp) がそのまま取れる
	rec = httptest.NewRecorder()
	fileHandler(rec, httptest.NewRequest("GET", "/api/file?"+q+"&path=GameA/shot2.bmp", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/bmp" || rec.Body.Len() == 0 {
		t.Fatalf("file bmp: code=%d ct=%s len=%d", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}
	// ダウンロード指定で Content-Disposition が付く
	rec = httptest.NewRecorder()
	fileHandler(rec, httptest.NewRequest("GET", "/api/file?"+q+"&path=GameA/shot1.jpg&dl=1", nil))
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "shot1.jpg") {
		t.Fatalf("dl header: %q", rec.Header().Get("Content-Disposition"))
	}

	// サムネイル (bmp → jpeg 化される)
	rec = httptest.NewRecorder()
	thumbHandler(rec, httptest.NewRequest("GET", "/api/thumb?"+q+"&path=GameB/pic.bmp", nil))
	if rec.Code != 200 {
		t.Fatalf("thumb code %d", rec.Code)
	}
	if _, format, err := image.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil || format != "jpeg" {
		t.Fatalf("thumb format=%s err=%v", format, err)
	}

	// ZIP 一括ダウンロード
	body := `{"ip":"127.0.0.1","port":` + strconv.Itoa(f.port) + `,"paths":["GameA/shot1.jpg","GameB/pic.bmp"]}`
	rec = httptest.NewRecorder()
	zipHandler(rec, httptest.NewRequest("POST", "/api/zip", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("zip code %d body %s", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 2 {
		t.Fatalf("zip contents: err=%v files=%d", err, len(zr.File))
	}

	// 削除
	body = `{"ip":"127.0.0.1","port":` + strconv.Itoa(f.port) + `,"paths":["GameB/pic.bmp"]}`
	rec = httptest.NewRecorder()
	deleteManyHandler(rec, httptest.NewRequest("POST", "/api/delete-many", strings.NewReader(body)))
	var dr struct {
		Deleted int      `json:"deleted"`
		Failed  []string `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dr); err != nil || dr.Deleted != 1 || len(dr.Failed) != 0 {
		t.Fatalf("delete: %v %+v body=%s", err, dr, rec.Body.String())
	}

	// 削除後の一覧は 3 件
	rec = httptest.NewRecorder()
	listHandler(rec, httptest.NewRequest("GET", "/api/list?"+q, nil))
	json.Unmarshal(rec.Body.Bytes(), &lr)
	if lr.Count != 3 {
		t.Fatalf("after delete count=%d", lr.Count)
	}
}

// ZIP の GET 形式 (リンク経由ダウンロード) とダウンロード状態の追跡
func TestZipGetAndDlStatus(t *testing.T) {
	f := newFakeFTP(t)
	port := strconv.Itoa(f.port)
	url := "/api/zip?ip=127.0.0.1&port=" + port + "&tok=tok123&p=" + "GameA/shot1.jpg&p=GameA/shot2.bmp"
	rec := httptest.NewRecorder()
	zipHandler(rec, httptest.NewRequest("GET", url, nil))
	if rec.Code != 200 {
		t.Fatalf("zip GET code %d body %s", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 2 {
		t.Fatalf("zip GET contents: err=%v files=%d", err, len(zr.File))
	}
	rec = httptest.NewRecorder()
	dlStatusHandler(rec, httptest.NewRequest("GET", "/api/dlstatus?tok=tok123", nil))
	if !strings.Contains(rec.Body.String(), `"done"`) {
		t.Fatalf("dlstatus = %s, want done", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	dlStatusHandler(rec, httptest.NewRequest("GET", "/api/dlstatus?tok=unknown", nil))
	if !strings.Contains(rec.Body.String(), `"none"`) {
		t.Fatalf("dlstatus unknown = %s, want none", rec.Body.String())
	}
	// エラーはコードで返る (画面側が言語に訳す)
	rec = httptest.NewRecorder()
	listHandler(rec, httptest.NewRequest("GET", "/api/list?ip=bad&port=1337", nil))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"badip"`) {
		t.Fatalf("badip: code=%d body=%s", rec.Code, rec.Body.String())
	}
}
