package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/image/bmp"
)

// ---- テスト用の偽 VitaShell FTP サーバー ----

type fakeFTP struct {
	mu    sync.Mutex
	files map[string][]byte
	ln    net.Listener
	port  int
}

func testImageJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{200, 100, 50, 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func testImageBMP() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{10, 180, 90, 255})
		}
	}
	var b bytes.Buffer
	bmp.Encode(&b, img)
	return b.Bytes()
}

func newFakeFTP(t *testing.T) *fakeFTP {
	t.Helper()
	f := &fakeFTP{files: map[string][]byte{}}
	base := "/ux0:/picture/SCREENSHOT"
	f.files[base+"/GameA/shot1.jpg"] = testImageJPEG()
	f.files[base+"/GameA/shot2.bmp"] = testImageBMP()
	f.files[base+"/GameA/ignore.txt"] = []byte("x")
	f.files[base+"/GameB/pic.bmp"] = testImageBMP()
	f.files[base+"/root.jpg"] = testImageJPEG()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.ln = ln
	f.port = ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return f
}

func (f *fakeFTP) dirs() map[string]bool {
	d := map[string]bool{"/": true, "/ux0:": true, "/ux0:/picture": true, "/ux0:/picture/SCREENSHOT": true}
	for p := range f.files {
		parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
		cur := ""
		for _, pt := range parts[:len(parts)-1] {
			cur += "/" + pt
			d[cur] = true
		}
	}
	return d
}

func (f *fakeFTP) serve(c net.Conn) {
	defer c.Close()
	rd := bufio.NewReader(c)
	var pasvLn net.Listener
	send := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
	send("220 FakeFTP ready")
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb, arg := line, ""
		if i := strings.Index(line, " "); i >= 0 {
			verb, arg = line[:i], line[i+1:]
		}
		switch strings.ToUpper(verb) {
		case "USER":
			send("331 password please")
		case "PASS":
			send("230 ok")
		case "TYPE":
			send("200 ok")
		case "CWD":
			f.mu.Lock()
			ok := f.dirs()[arg]
			f.mu.Unlock()
			if ok {
				send("250 ok")
			} else {
				send("550 no such dir")
			}
		case "PASV":
			if pasvLn != nil {
				pasvLn.Close()
			}
			pasvLn, _ = net.Listen("tcp", "127.0.0.1:0")
			p := pasvLn.Addr().(*net.TCPAddr).Port
			send(fmt.Sprintf("227 Entering Passive Mode (127,0,0,1,%d,%d)", p/256, p%256))
		case "MLSD":
			send("502 not implemented")
		case "LIST":
			f.mu.Lock()
			var lines []string
			prefix := strings.TrimSuffix(arg, "/") + "/"
			seen := map[string]bool{}
			for p, data := range f.files {
				if !strings.HasPrefix(p, prefix) {
					continue
				}
				rest := strings.TrimPrefix(p, prefix)
				seg := strings.SplitN(rest, "/", 2)[0]
				if seen[seg] {
					continue
				}
				seen[seg] = true
				if len(strings.SplitN(rest, "/", 2)) == 2 {
					lines = append(lines, fmt.Sprintf("drwxr-xr-x 1 0 0 0 Jan 01 2026 %s", seg))
				} else {
					lines = append(lines, fmt.Sprintf("-rw-r--r-- 1 0 0 %d Jan 01 2026 %s", len(data), seg))
				}
			}
			f.mu.Unlock()
			if pasvLn == nil {
				send("425 no pasv")
				continue
			}
			send("150 opening")
			dc, err := pasvLn.Accept()
			pasvLn.Close()
			pasvLn = nil
			if err != nil {
				send("425 fail")
				continue
			}
			for _, l := range lines {
				fmt.Fprintf(dc, "%s\r\n", l)
			}
			dc.Close()
			send("226 done")
		case "RETR":
			f.mu.Lock()
			data, ok := f.files[arg]
			f.mu.Unlock()
			if !ok || pasvLn == nil {
				send("550 no such file")
				continue
			}
			send("150 opening")
			dc, err := pasvLn.Accept()
			pasvLn.Close()
			pasvLn = nil
			if err != nil {
				send("425 fail")
				continue
			}
			dc.Write(data)
			dc.Close()
			send("226 done")
		case "SIZE":
			f.mu.Lock()
			data, ok := f.files[arg]
			f.mu.Unlock()
			if ok {
				send("213 " + strconv.Itoa(len(data)))
			} else {
				send("550 no")
			}
		case "DELE":
			f.mu.Lock()
			_, ok := f.files[arg]
			if ok {
				delete(f.files, arg)
			}
			f.mu.Unlock()
			if ok {
				send("250 deleted")
			} else {
				send("550 no")
			}
		case "QUIT":
			send("221 bye")
			return
		default:
			send("502 unknown")
		}
	}
}

// ---- テスト ----

func TestWalkCollectsJpgAndBmpByFolder(t *testing.T) {
	f := newFakeFTP(t)
	s, err := vitaConnect("127.0.0.1", f.port)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer s.close()
	if s.root != "/ux0:/picture/SCREENSHOT" {
		t.Fatalf("root = %q", s.root)
	}
	items, err := s.walk()
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("items = %d (%v), want 4 (txt excluded)", len(items), items)
	}
	byPath := map[string]galItem{}
	for _, it := range items {
		byPath[it.Path] = it
	}
	if it := byPath["GameA/shot1.jpg"]; it.Ext != "jpg" || it.Folder != "GameA" || it.Size == 0 {
		t.Errorf("shot1: %+v", it)
	}
	if it := byPath["GameA/shot2.bmp"]; it.Ext != "bmp" {
		t.Errorf("shot2: %+v", it)
	}
	if it := byPath["GameB/pic.bmp"]; it.Ext != "bmp" || it.Folder != "GameB" {
		t.Errorf("pic: %+v", it)
	}
	if it := byPath["root.jpg"]; it.Folder != "" {
		t.Errorf("root.jpg folder = %q, want empty", it.Folder)
	}
	// 名前順: フォルダ昇順 ("" が先頭)
	if items[0].Folder != "" || items[1].Folder != "GameA" {
		t.Errorf("sort order: %+v", items)
	}
}

func TestThumbJpgAndBmp(t *testing.T) {
	f := newFakeFTP(t)
	s, err := vitaConnect("127.0.0.1", f.port)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	for _, rel := range []string{"GameA/shot1.jpg", "GameA/shot2.bmp", "GameB/pic.bmp"} {
		b, err := makeThumb(s, s.abs(rel), "test|"+rel)
		if err != nil {
			t.Fatalf("thumb %s: %v", rel, err)
		}
		img, format, err := image.Decode(bytes.NewReader(b))
		if err != nil || format != "jpeg" {
			t.Fatalf("thumb %s decode: format=%s err=%v", rel, format, err)
		}
		if img.Bounds().Dx() == 0 {
			t.Errorf("thumb %s empty", rel)
		}
	}
}

func TestDelete(t *testing.T) {
	f := newFakeFTP(t)
	s, err := vitaConnect("127.0.0.1", f.port)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if err := s.dele(s.abs("GameB/pic.bmp")); err != nil {
		t.Fatalf("dele: %v", err)
	}
	f.mu.Lock()
	_, ok := f.files["/ux0:/picture/SCREENSHOT/GameB/pic.bmp"]
	f.mu.Unlock()
	if ok {
		t.Error("file still exists after DELE")
	}
	if err := s.dele(s.abs("GameB/pic.bmp")); err == nil {
		t.Error("second DELE should fail")
	}
}

func TestCleanRel(t *testing.T) {
	if _, ok := cleanRel("../secret.jpg"); ok {
		t.Error("traversal accepted")
	}
	if _, ok := cleanRel("GameA/shot.txt"); ok {
		t.Error("txt accepted")
	}
	if c, ok := cleanRel("GameA/shot1.jpg"); !ok || c != "GameA/shot1.jpg" {
		t.Errorf("cleanRel jpg: %q %v", c, ok)
	}
}
