# Vita Gallery

## 日本語

PS Vita のスクリーンショットを、VitaShell の FTP 経由で PC / スマホから見たり、ダウンロードしたり、削除したりするソフトです。

kaiHEN Installer と同じ方式（Go サーバー + HTML GUI）で作ってあります。

### 対象

場所は `ux0:picture/SCREENSHOT` の中です。

その中にあるフォルダ別のファイルと、直下のファイルが対象です。

形式は `.jpg` / `.bmp` です。

接続先は VitaShell の FTP サーバーです（既定ポート 1337、anonymous ログイン）。

### できること

- フォルダ別のサムネイル一覧、フォルダでの絞り込み、「更新」で再読み込み
- クリックで拡大表示（前へ / 次へ）
- ダウンロードは 1枚ずつ、または選択分をまとめて ZIP でできます

ダウンロード中は「ダウンロードしています…」と表示されます。

- 削除もできます（1枚ずつ / 選択分をまとめて）

確認ダイアログが出ます。

Vita 側のファイルが実際に消えます。

- 12言語対応です（日本語 / English / Français / Deutsch / Italiano / Español / Русский / Português / Polski / 한국어 / 简体中文 / 繁體中文）

既定はシステム言語です。

切替は保存されます。

- IP アドレスは前回分を自動で覚えます

「IPを保存」で一覧に登録して選ぶこともできます（kaiHEN Installer と同じ方式）。

- 自己更新できます

新しいリリースが出ると起動時に知らせ、そのまま更新できます（kaiHEN Installer と同じ方式）。

Windows は exe 差し替え＋再起動します。

Android は APK を落としてインストール画面を開きます。

### 使い方

1. Windows 版は `Vita Gallery.exe` を起動します。Android 版はアプリを開きます
2. PS Vita で VitaShell を開き、**SELECT** を押して FTP を開始します
3. 画面に表示された IP アドレスを入れて「読み込む」を押します

PC / スマホと PS Vita が同じ Wi-Fi（同じネットワーク）につながっている必要があります。

### ダウンロード

[Releases](https://github.com/yyoossk/vita-gallery/releases) から入手できます。

- `VitaGallery-<stamp>.zip` — Windows 版（`Vita Gallery.exe` + README.txt）
- `VitaGallery-Android.apk` — Android 版（arm64）

インストール不要です。

Android 版は提供元不明アプリの許可が要ります。

### ビルド

Windows (Go 1.24+):

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.buildStamp=YYYYMMDD-HHMM" .
```

Android は `android/build.sh` を参照してください（Go / JDK / Android SDK build-tools 34 が必要）。

署名鍵 (keystore.jks) はリポジトリに含みません。

### ライセンス

MIT — [LICENSE](LICENSE) を参照してください。

## English

Vita Gallery lets you view, download and delete PS Vita screenshots over VitaShell FTP, from a PC or a phone.

It is built the same way as kaiHEN Installer (a small Go server + HTML GUI).

### Scope

The files live in `ux0:picture/SCREENSHOT`.

The files in the folders inside it, and the files directly under it, are shown.

The formats are `.jpg` / `.bmp`.

It connects to the VitaShell FTP server (default port 1337, anonymous login).

### Features

- A thumbnail list grouped by folder, a folder filter, and a Refresh button
- Click a photo to view it larger (Previous / Next)
- Download one photo at a time, or the selected photos together as a ZIP

A "Downloading…" message is shown while downloading.

- Deleting is supported (one at a time / all selected together)

A confirmation dialog appears first.

The file on the Vita itself is actually deleted.

- 12 languages are supported (Japanese / English / Français / Deutsch / Italiano / Español / Русский / Português / Polski / 한국어 / 简体中文 / 繁體中文)

The system language is the default.

Your choice is remembered.

- The IP address is remembered from the last time

You can also register it in a list with "Save IP" and pick it later (the same way as kaiHEN Installer).

- Self-update is supported

When a new release comes out, the app tells you at startup and updates in place (the same way as kaiHEN Installer).

On Windows, the exe is replaced and the app restarts.

On Android, the APK is downloaded and the install screen opens.

### Usage

1. On Windows, start `Vita Gallery.exe`. On Android, open the app
2. Open VitaShell on your PS Vita and press **SELECT** to start FTP
3. Enter the IP address shown and press "Load"

Your PC/phone and PS Vita must be on the same Wi-Fi (the same network).

### Downloads

Get them from [Releases](https://github.com/yyoossk/vita-gallery/releases).

- `VitaGallery-<stamp>.zip` — Windows version (`Vita Gallery.exe` + README.txt)
- `VitaGallery-Android.apk` — Android version (arm64)

No installation is needed.

On Android, installing from unknown sources must be allowed.

### Building

Windows (Go 1.24+):

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.buildStamp=YYYYMMDD-HHMM" .
```

For Android, see `android/build.sh` (Go / JDK / Android SDK build-tools 34 required).

The signing key (keystore.jks) is not part of this repository.

### License

MIT — see [LICENSE](LICENSE).
