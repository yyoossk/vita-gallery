Vita Gallery (build 20261123-1856)
==================================

日本語
------

PS Vita のスクリーンショットを、VitaShell の FTP 経由で PC / スマホから見たり、ダウンロードしたり、削除したりするソフトです。
kaiHEN Installer と同じ方式 (単体 exe + 画面) で作ってあります。
インストールは不要です。
Android 版 (Vita Gallery.apk) もあります。

言語

12言語対応です (日本語 / English / Français / Deutsch / Italiano / Español / Русский / Português / Polski / 한국어 / 简体中文 / 繁體中文)。
最初はシステムの言語で表示されます。
画面右上のプルダウンで切り替えると、その選択を覚えます。

対象のファイル

場所は ux0:picture/SCREENSHOT の中です。
その中にあるフォルダ別の .jpg / .bmp ファイルが対象です。
SCREENSHOT 直下のファイルも表示します。

使い方

1. 「Vita Gallery.exe」を起動します (Android 版はアプリを開きます)。
2. PS Vita で VitaShell を開きます。
3. VitaShell で SELECT ボタンを押して FTP を開始します。
画面に IP アドレスが表示されます。
ポートは通常 1337 です。
4. 画面で、表示された IP アドレスを入れて「読み込む」を押します。
PC / スマホと PS Vita が同じ Wi-Fi につながっている必要があります。

できること

- フォルダ別のサムネイル一覧 (フォルダで絞り込みもできます)
- クリックで拡大表示 (前へ / 次へ で移動できます)
- 1枚ずつダウンロード (ダウンロード中は「ダウンロードしています…」と表示します)
- チェックを入れた写真をまとめてダウンロード (ZIP)
- 削除 (1枚ずつ / チェックを入れた分をまとめて)

削除すると Vita 側のファイルが消えます。
元に戻せません。
削除の前に確認画面が出ます。

IP アドレスの保存

入れた IP アドレスは、次回もそのまま表示されます。
「IPを保存」を押すと一覧に登録できます。
登録した IP は「保存済み IP」から選んだり、削除したりできます。

更新

新しい版が GitHub に公開されると、起動時に「新しい版があります」と表示されます。
「更新する」を押すと、そのまま更新できます。
Windows 版は exe を差し替えて自動で再起動します。
Android 版は APK をダウンロードして、インストール画面が開きます。

注意

- VitaShell の FTP を止める (START を押す) と接続できなくなります
- IP アドレスと言語は前回入れたものを覚えます


English
-------

Vita Gallery lets you view, download and delete PS Vita screenshots over VitaShell FTP, from a PC or a phone.
It is built the same way as kaiHEN Installer (a single exe + screen).
No installation is needed.
An Android version (Vita Gallery.apk) is also available.

Languages

12 languages are supported (Japanese / English / Français / Deutsch / Italiano / Español / Русский / Português / Polski / 한국어 / 简体中文 / 繁體中文).
The system language is used at first.
When you switch languages in the top-right menu, your choice is remembered.

Files

The files live in ux0:picture/SCREENSHOT.
The .jpg / .bmp files in the folders inside it are shown.
Files directly under SCREENSHOT are shown too.

How to use

1. Start "Vita Gallery.exe" (on Android, open the app).
2. Open VitaShell on your PS Vita.
3. Press SELECT in VitaShell to start FTP.
The IP address appears on the Vita screen.
The port is usually 1337.
4. Enter the IP address shown and press "Load".
Your PC/phone and PS Vita must be on the same Wi-Fi (the same network).

Features

- A thumbnail list grouped by folder (you can filter by folder)
- Click a photo to view it larger (move with Previous / Next)
- Download one photo at a time (a "Downloading…" message is shown while downloading)
- Download the checked photos together (ZIP)
- Delete (one at a time / all checked together)

Deleting removes the file on the Vita itself.
It cannot be undone.
A confirmation dialog appears before deleting.

Saving IP addresses

The IP address you entered is shown again on the next launch.
Press "Save IP" to register it in a list.
You can pick or delete registered IPs from "Saved IP".

Updates

When a new version is published on GitHub, "An update is available" is shown at startup.
Press "Update now" to update in place.
On Windows, the exe is replaced and the app restarts automatically.
On Android, the APK is downloaded and the install screen opens.

Notes

- If you stop FTP in VitaShell (press START), the app can no longer connect
- The IP address and language are remembered from the last time
