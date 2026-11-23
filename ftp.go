package main

// VitaShell FTP クライアント。
// VitaShell は SELECT で FTP サーバー (既定ポート 1337) を開始する。
// 認証は anonymous で通る。ux0: はサーバーにより
// "/ux0:/..." 形式でも "/ux0/..." 形式でも見えるため、
// SCREENSHOT フォルダへの CWD / LIST が成功した表記を採用する。

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const screenshotRootBase = "picture/SCREENSHOT"

var rootCandidates = []string{
	"/ux0:/picture/SCREENSHOT",
	"ux0:/picture/SCREENSHOT",
	"/ux0/picture/SCREENSHOT",
	"/picture/SCREENSHOT",
}

type ftpReader struct {
	conn net.Conn
	buf  []byte
}

func newFTPReader(c net.Conn) *ftpReader { return &ftpReader{conn: c} }

func (f *ftpReader) readLine() (string, error) {
	for {
		if i := bytes.IndexByte(f.buf, '\n'); i >= 0 {
			line := string(f.buf[:i+1])
			f.buf = f.buf[i+1:]
			return strings.TrimRight(line, "\r\n"), nil
		}
		tmp := make([]byte, 4096)
		n, err := f.conn.Read(tmp)
		if n > 0 {
			f.buf = append(f.buf, tmp[:n]...)
		}
		if err != nil {
			return "", err
		}
	}
}

func (f *ftpReader) readReply() (int, string) {
	line, err := f.readLine()
	if err != nil {
		return 0, ""
	}
	code := 0
	fmt.Sscanf(line, "%d", &code)
	if len(line) > 3 && line[3] == '-' {
		for {
			l2, err := f.readLine()
			if err != nil {
				break
			}
			if len(l2) > 3 {
				c2 := 0
				fmt.Sscanf(l2, "%d", &c2)
				if c2 == code && l2[3] == ' ' {
					line = l2
					break
				}
			}
		}
	}
	return code, line
}

// lanDial は LAN 向けのダイヤル。Android シェルが VITA_LAN_IP
// (Wi-Fi のアドレス) を渡した場合はそこに束縛する: インターネットの
// 無い Wi-Fi ではモバイルデータが既定経路になり、束縛しないと
// LAN に届かない (kaiHEN Installer と同じ対策)。失敗時は束縛なしで再試行。
func lanDial(addr string, timeout time.Duration) (net.Conn, error) {
	if ip := net.ParseIP(strings.TrimSpace(os.Getenv("VITA_LAN_IP"))); ip != nil && ip.To4() != nil {
		d := &net.Dialer{LocalAddr: &net.TCPAddr{IP: ip}, Timeout: timeout}
		if conn, err := d.Dial("tcp", addr); err == nil {
			return conn, nil
		}
	}
	return net.DialTimeout("tcp", addr, timeout)
}

type vitaSession struct {
	ctrl net.Conn
	rd   *ftpReader
	root string // 確定した SCREENSHOT フォルダの絶対表記
}

func (s *vitaSession) cmd(c string) (int, string) {
	fmt.Fprintf(s.ctrl, "%s\r\n", c)
	return s.rd.readReply()
}

func (s *vitaSession) close() {
	s.cmd("QUIT")
	s.ctrl.Close()
}

func vitaConnect(ip string, port int) (*vitaSession, error) {
	if port <= 0 || port > 65535 {
		port = 1337
	}
	ctrl, err := lanDial(net.JoinHostPort(ip, strconv.Itoa(port)), 8*time.Second)
	if err != nil {
		return nil, err
	}
	s := &vitaSession{ctrl: ctrl}
	ctrl.SetDeadline(time.Now().Add(600 * time.Second))
	s.rd = newFTPReader(ctrl)
	if code, _ := s.rd.readReply(); code != 220 {
		ctrl.Close()
		return nil, errors.New("no greeting")
	}
	code, _ := s.cmd("USER anonymous")
	if code == 331 {
		code, _ = s.cmd("PASS guest@")
	}
	if code != 230 && code != 202 {
		// VitaShell の実装によっては任意のユーザー名を受け付ける
		code2, _ := s.cmd("USER vitashell")
		if code2 == 331 {
			code2, _ = s.cmd("PASS vitashell")
		}
		if code2 != 230 && code2 != 202 {
			ctrl.Close()
			return nil, errors.New("login refused")
		}
	}
	s.cmd("TYPE I")
	root, err := s.resolveRoot()
	if err != nil {
		ctrl.Close()
		return nil, err
	}
	s.root = root
	return s, nil
}

// resolveRoot は SCREENSHOT フォルダの表記を確定する。
// まず CWD、ダメなら LIST の成功で判定する (CWD を受け付けない
// サーバー実装があるため)。
func (s *vitaSession) resolveRoot() (string, error) {
	for _, c := range rootCandidates {
		if code, _ := s.cmd("CWD " + c); code == 250 {
			s.cmd("CWD /")
			return c, nil
		}
	}
	for _, c := range rootCandidates {
		if _, err := s.list(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("no SCREENSHOT folder (ux0:picture/SCREENSHOT が見つかりません)")
}

func (s *vitaSession) pasv() (net.Conn, error) {
	code, line := s.cmd("PASV")
	if code != 227 {
		return nil, fmt.Errorf("PASV: %d", code)
	}
	i := strings.Index(line, "(")
	j := strings.Index(line, ")")
	if i < 0 || j <= i {
		return nil, errors.New("PASV: bad reply")
	}
	parts := strings.Split(line[i+1:j], ",")
	if len(parts) != 6 {
		return nil, errors.New("PASV: bad reply")
	}
	n := make([]int, 6)
	for k, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, errors.New("PASV: bad reply")
		}
		n[k] = v
	}
	host := fmt.Sprintf("%d.%d.%d.%d", n[0], n[1], n[2], n[3])
	return lanDial(net.JoinHostPort(host, strconv.Itoa(n[4]*256+n[5])), 10*time.Second)
}

func (s *vitaSession) transfer(cmd string) (net.Conn, error) {
	dc, err := s.pasv()
	if err != nil {
		return nil, err
	}
	code, _ := s.cmd(cmd)
	if code != 150 && code != 125 {
		dc.Close()
		return nil, fmt.Errorf("%s: %d", cmd, code)
	}
	return dc, nil
}

func (s *vitaSession) finish() { s.rd.readReply() }

type ftpEntry struct {
	Name string
	Dir  bool
	Size int64
}

func (s *vitaSession) list(abs string) ([]ftpEntry, error) {
	if ents, err := s.listCmd("MLSD " + abs); err == nil && len(ents) > 0 {
		return ents, nil
	}
	return s.listCmd("LIST " + abs)
}

func (s *vitaSession) listCmd(cmd string) ([]ftpEntry, error) {
	dc, err := s.transfer(cmd)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(dc, 16<<20))
	dc.Close()
	s.finish()
	if err != nil {
		return nil, err
	}
	var out []ftpEntry
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(cmd, "MLSD") {
			if e, ok := parseMLSDLine(line); ok {
				out = append(out, e)
			}
		} else {
			if e, ok := parseLISTLineAny(line); ok {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func parseMLSDLine(line string) (ftpEntry, bool) {
	i := strings.Index(line, " ")
	if i < 0 {
		return ftpEntry{}, false
	}
	facts, name := line[:i], strings.TrimSpace(line[i+1:])
	if name == "." || name == ".." || name == "" {
		return ftpEntry{}, false
	}
	e := ftpEntry{Name: name}
	for _, f := range strings.Split(facts, ";") {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "type":
			e.Dir = strings.EqualFold(kv[1], "dir")
		case "size":
			e.Size, _ = strconv.ParseInt(kv[1], 10, 64)
		}
	}
	return e, true
}

// parseLISTLineAny は unix 形式と DOS 形式の両方の LIST を解釈する。
func parseLISTLineAny(line string) (ftpEntry, bool) {
	if e, ok := parseUnixLISTLine(line); ok {
		return e, true
	}
	return parseDOSLISTLine(line)
}

func parseUnixLISTLine(line string) (ftpEntry, bool) {
	if len(line) < 2 || (line[0] != '-' && line[0] != 'd' && line[0] != 'l') {
		return ftpEntry{}, false
	}
	f := strings.Fields(line)
	if len(f) < 9 {
		return ftpEntry{}, false
	}
	size, err := strconv.ParseInt(f[4], 10, 64)
	if err != nil {
		return ftpEntry{}, false
	}
	name := strings.Join(f[8:], " ")
	if name == "." || name == ".." {
		return ftpEntry{}, false
	}
	return ftpEntry{Name: name, Dir: line[0] == 'd', Size: size}, true
}

// DOS 形式: "01-15-26  01:23PM       <DIR>          folder" /
//           "01-15-26  01:23PM              12345 file.jpg"
func parseDOSLISTLine(line string) (ftpEntry, bool) {
	f := strings.Fields(line)
	if len(f) < 4 {
		return ftpEntry{}, false
	}
	if strings.EqualFold(f[2], "<DIR>") {
		name := strings.Join(f[3:], " ")
		if name == "." || name == ".." {
			return ftpEntry{}, false
		}
		return ftpEntry{Name: name, Dir: true}, true
	}
	size, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil {
		return ftpEntry{}, false
	}
	name := strings.Join(f[3:], " ")
	if name == "." || name == ".." {
		return ftpEntry{}, false
	}
	return ftpEntry{Name: name, Size: size}, true
}

func (s *vitaSession) retr(abs string, w io.Writer) error {
	dc, err := s.transfer("RETR " + abs)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, dc)
	dc.Close()
	s.finish()
	return err
}

func (s *vitaSession) size(abs string) (int64, bool) {
	code, line := s.cmd("SIZE " + abs)
	if code != 213 {
		return 0, false
	}
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0, false
	}
	n, err := strconv.ParseInt(f[len(f)-1], 10, 64)
	return n, err == nil
}

func (s *vitaSession) dele(abs string) error {
	code, _ := s.cmd("DELE " + abs)
	if code != 250 {
		return fmt.Errorf("DELE: %d", code)
	}
	return nil
}

// abs は SCREENSHOT ルートからの相対パス (folder/file.jpg) を絶対表記にする。
func (s *vitaSession) abs(rel string) string {
	return strings.TrimSuffix(s.root, "/") + "/" + rel
}
