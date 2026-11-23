# Vita Gallery

PS Vita のスクリーンショットを、VitaShell の FTP 経由で PC / スマホから見たり、ダウンロードしたり、削除したりするソフトです。[kaiHEN Installer](https://github.com/yyoossk/kaihen-installer) と同じ方式（Go サーバー + HTML GUI）で作ってあります。

Browse, download and delete PS Vita screenshots over VitaShell FTP, from a PC or a phone. Built the same way as kaiHEN Installer (a small Go server + HTML GUI).

## 対象 / Scope

- 場所: `ux0:picture/SCREENSHOT` の中（フォルダ別のファイルと、直下のファイル）
- 形式: `.jpg` / `.bmp`
- 接続先: VitaShell の FTP サーバー（既定ポート 1337、anonymous ログイン）

## できること / Features

- フォルダ別のサムネイル一覧、フォルダでの絞り込み、「更新」で再読み込み
- クリックで拡大表示（前へ / 次へ）
- ダウンロード: 1枚ずつ（「ダウンロードしています…」表示つき）/ 選択分をまとめて ZIP
- 削除: 1枚ずつ / 選択分をまとめて（確認ダイアログつき。Vita 側のファイルが実際に消えます）
- 12言語対応（日本語 / English / Français / Deutsch / Italiano / Español / Русский / Português / Polski / 한국어 / 简体中文 / 繁體中文）。既定はシステム言語、切替は保存されます
- IP アドレスは前回分を自動で覚えるほか、「IPを保存」で一覧に登録して選べます（kaiHEN Installer と同じ方式）
- 自己更新: 新しいリリースが出ると起動時に知らせ、そのまま更新できます（kaiHEN Installer と同じ方式。Windows は exe 差し替え＋再起動、Android は APK を落としてインストール画面を開きます）

## 使い方 / Usage

1. Windows 版は `Vita Gallery.exe` を起動、Android 版はアプリを開く
2. PS Vita で VitaShell を開き、**SELECT** を押して FTP を開始する
3. 画面に表示された IP アドレスを入れて「読み込む」を押す

PC / スマホと PS Vita が同じ Wi-Fi（同じネットワーク）につながっている必要があります。

## ダウンロード / Downloads

[Releases](https://github.com/yyoossk/vita-gallery/releases) から:

- `VitaGallery-<stamp>.zip` — Windows 版（`Vita Gallery.exe` + README.txt）
- `VitaGallery-Android.apk` — Android 版（arm64）

インストール不要です。Android 版は提供元不明アプリの許可が要ります。

## ビルド / Building

Windows (Go 1.24+):

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.buildStamp=YYYYMMDD-HHMM" .
```

Android: `android/build.sh` を参照（Go / JDK / Android SDK build-tools 34 が必要）。署名鍵 (keystore.jks) はリポジトリに含みません。

## ライセンス / License

MIT — see [LICENSE](LICENSE).
