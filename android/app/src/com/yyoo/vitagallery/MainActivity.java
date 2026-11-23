package com.yyoo.vitagallery;

import android.app.Activity;
import android.app.DownloadManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageInstaller;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.net.Uri;
import android.net.wifi.WifiInfo;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.provider.Settings;
import android.view.WindowManager;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Toast;

import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.URL;
import java.util.List;
import java.util.Map;

// Vita Gallery for Android: a thin WebView shell around the same Go
// server and HTML GUI as the Windows build. The Go server ships as a
// native library (libvitagallery.so) and is started as a process; the
// page it serves is the whole UI. Downloads the page starts are handed
// to the system DownloadManager so files land in Downloads.
// ASCII-only comments: javac here reads sources as US-ASCII.
public class MainActivity extends Activity {
    private static final int[] PORTS = {47660, 47661, 47662, 47663, 47664};

    private WebView web;
    private Process server;
    private volatile boolean loaded = false;
    private volatile boolean destroying = false;

    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        web = new WebView(this);
        WebSettings s = web.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(false);
        web.addJavascriptInterface(new VitaBridge(), "VitaBridge");
        web.setDownloadListener((url, ua, disp, mime, len) -> startDownload(url, disp, mime));
        web.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageFinished(WebView view, String url) {
                loaded = true;
            }
        });
        setContentView(web);
        handleInstallerStatus(getIntent());
        new Thread(this::startServerAndLoad, "vita-starter").start();
    }

    private void startServerAndLoad() {
        int port = probePort(300);
        if (port > 0 && !serverIsCurrent(port)) {
            // A server from an older install may still be running after
            // an app update; it serves an old page, so replace it.
            quitServer(port);
            long deadline = System.currentTimeMillis() + 5000;
            while (System.currentTimeMillis() < deadline && probePort(200) > 0) {
                try {
                    Thread.sleep(200);
                } catch (InterruptedException e) {
                    break;
                }
            }
            port = probePort(200);
        }
        if (port < 0) {
            try {
                startServer();
            } catch (IOException e) {
                toast("Failed to start the server: " + e.getMessage());
                return;
            }
            port = probePort(20000);
        }
        if (port < 0) {
            toast("The server did not start. Please reopen the app.");
            return;
        }
        final String url = "http://127.0.0.1:" + port + "/";
        runOnUiThread(() -> web.loadUrl(url));
    }

    private int probePort(long timeoutMs) {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (true) {
            for (int p : PORTS) {
                try (Socket sock = new Socket()) {
                    sock.connect(new InetSocketAddress("127.0.0.1", p), 250);
                    return p;
                } catch (IOException ignored) {
                }
            }
            if (System.currentTimeMillis() >= deadline) return -1;
            try {
                Thread.sleep(200);
            } catch (InterruptedException e) {
                return -1;
            }
        }
    }

    private String appVersion() {
        try {
            return getPackageManager().getPackageInfo(getPackageName(), 0).versionName;
        } catch (Exception e) {
            return "unknown";
        }
    }

    // True when the answering server serves this build's page (the top
    // bar carries the same build stamp as the APK version name).
    private boolean serverIsCurrent(int port) {
        String version = appVersion();
        if (version == null || version.equals("unknown")) return true;
        HttpURLConnection c = null;
        try {
            c = (HttpURLConnection) new URL("http://127.0.0.1:" + port + "/").openConnection();
            c.setConnectTimeout(1500);
            c.setReadTimeout(1500);
            InputStream in = c.getInputStream();
            byte[] buf = new byte[65536];
            int n = 0;
            while (n < buf.length) {
                int r = in.read(buf, n, buf.length - n);
                if (r < 0) break;
                n += r;
            }
            return new String(buf, 0, n, "UTF-8").contains("build " + version);
        } catch (Exception e) {
            return true;
        } finally {
            if (c != null) c.disconnect();
        }
    }

    private void quitServer(int port) {
        HttpURLConnection c = null;
        try {
            c = (HttpURLConnection) new URL("http://127.0.0.1:" + port + "/api/quit").openConnection();
            c.setRequestMethod("POST");
            c.setConnectTimeout(1500);
            c.setReadTimeout(1500);
            c.getResponseCode();
        } catch (Exception ignored) {
        } finally {
            if (c != null) c.disconnect();
        }
    }

    private void startServer() throws IOException {
        String nativeDir = getApplicationInfo().nativeLibraryDir;
        File filesDir = getFilesDir();
        ProcessBuilder pb = new ProcessBuilder(nativeDir + "/libvitagallery.so");
        Map<String, String> env = pb.environment();
        env.put("LD_LIBRARY_PATH", nativeDir);
        env.put("HOME", filesDir.getAbsolutePath());
        env.put("TMPDIR", getCacheDir().getAbsolutePath());
        File ext = getExternalFilesDir(null);
        if (ext != null) env.put("VITA_EXTERNAL_DIR", ext.getAbsolutePath());
        String dns = dnsServers();
        if (!dns.isEmpty()) env.put("VITA_DNS", dns);
        String lan = wifiLanIp();
        if (!lan.isEmpty()) env.put("VITA_LAN_IP", lan);
        String path = env.get("PATH");
        env.put("PATH", "/system/bin" + (path == null || path.isEmpty() ? "" : ":" + path));
        pb.redirectErrorStream(true);
        pb.redirectOutput(new File(filesDir, "server.log"));
        server = pb.start();
        new Thread(() -> {
            try {
                server.waitFor();
            } catch (InterruptedException ignored) {
            }
            // The page's Quit button stops the server; mirror that by
            // closing the app once the first page has loaded.
            if (!destroying && loaded && !isFinishing()) {
                runOnUiThread(this::finish);
            }
        }, "vita-watchdog").start();
    }

    // Save a file the page asked to download with the system
    // DownloadManager (public Downloads folder on Android 10+).
    private void startDownload(String url, String disp, String mime) {
        String name = "download";
        if (disp != null) {
            int i = disp.indexOf("filename=");
            if (i >= 0) {
                String v = disp.substring(i + 9).replace("\"", "").trim();
                int sc = v.indexOf(';');
                if (sc >= 0) v = v.substring(0, sc).trim();
                if (!v.isEmpty()) name = v;
            }
        }
        try {
            DownloadManager.Request req = new DownloadManager.Request(Uri.parse(url));
            req.setTitle(name);
            if (mime != null && !mime.isEmpty()) req.setMimeType(mime);
            req.setNotificationVisibility(
                    DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
            if (Build.VERSION.SDK_INT >= 29) {
                req.setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name);
            } else {
                req.setDestinationInExternalFilesDir(this, Environment.DIRECTORY_DOWNLOADS, name);
            }
            DownloadManager dm = (DownloadManager) getSystemService(DOWNLOAD_SERVICE);
            dm.enqueue(req);
        } catch (Exception e) {
            toast("Download failed");
        }
    }

    private void toast(String msg) {
        runOnUiThread(() -> Toast.makeText(this, msg, Toast.LENGTH_LONG).show());
    }

    @Override
    public void onBackPressed() {
        if (web == null || !loaded) {
            super.onBackPressed();
            return;
        }
        // The page handles back first: close a dialog or viewer, or go
        // from the gallery back to the home page. Only on the home page
        // does the default (leaving the app) apply.
        web.evaluateJavascript(
                "(function(){return (window.__vitaBack&&window.__vitaBack())?'handled':''})()",
                value -> {
                    if (value == null || !value.contains("handled")) {
                        MainActivity.super.onBackPressed();
                    }
                });
    }

    // The phone's DNS servers for the active network, comma-joined:
    // Android has no /etc/resolv.conf, so the Go server cannot resolve
    // names (github.com for the update check) without them.
    private String dnsServers() {
        try {
            ConnectivityManager cm = (ConnectivityManager) getSystemService(CONNECTIVITY_SERVICE);
            Network n = cm == null ? null : cm.getActiveNetwork();
            LinkProperties lp = n == null || cm == null ? null : cm.getLinkProperties(n);
            if (lp == null) return "";
            StringBuilder sb = new StringBuilder();
            List<java.net.InetAddress> list = lp.getDnsServers();
            for (int i = 0; i < list.size(); i++) {
                if (i > 0) sb.append(',');
                sb.append(list.get(i).getHostAddress());
            }
            return sb.toString();
        } catch (Exception e) {
            return "";
        }
    }

    // This phone's IPv4 address on the Wi-Fi network, or "". The Go
    // server binds its LAN dials to it so VitaShell FTP traffic stays
    // on Wi-Fi even when mobile data is the default route.
    private String wifiLanIp() {
        try {
            WifiManager wm = (WifiManager) getApplicationContext().getSystemService(WIFI_SERVICE);
            WifiInfo info = wm == null ? null : wm.getConnectionInfo();
            int ip = info == null ? 0 : info.getIpAddress();
            if (ip == 0) return "";
            return (ip & 0xff) + "." + ((ip >> 8) & 0xff) + "." + ((ip >> 16) & 0xff) + "." + ((ip >> 24) & 0xff);
        } catch (Exception e) {
            return "";
        }
    }

    // ---- self-update install (same flow as kaiHEN Installer) ----

    // JavaScript bridge for the update flow. It MUST be a named public
    // class: an anonymous @JavascriptInterface class is invisible to
    // JavaScript and the page's calls silently do nothing.
    public class VitaBridge {
        @JavascriptInterface
        public boolean canInstall() {
            return Build.VERSION.SDK_INT < 26 || getPackageManager().canRequestPackageInstalls();
        }

        @JavascriptInterface
        public void requestInstallPermission() {
            if (Build.VERSION.SDK_INT >= 26 && !getPackageManager().canRequestPackageInstalls()) {
                Intent it = new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                        Uri.parse("package:" + getPackageName()));
                it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                startActivity(it);
            }
        }

        @JavascriptInterface
        public void installUpdate() {
            runOnUiThread(() -> startUpdateInstall());
        }
    }

    private void startUpdateInstall() {
        File ext = getExternalFilesDir(null);
        File apk = ext == null ? null : new File(ext, "vita-update.apk");
        if (apk == null || !apk.exists()) {
            toast("Update file not found");
            return;
        }
        PackageInstaller pi = getPackageManager().getPackageInstaller();
        PackageInstaller.Session session = null;
        try {
            PackageInstaller.SessionParams params =
                    new PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL);
            int id = pi.createSession(params);
            session = pi.openSession(id);
            try (InputStream in = new FileInputStream(apk);
                 OutputStream out = session.openWrite("vita-update", 0, apk.length())) {
                byte[] buf = new byte[65536];
                int n;
                while ((n = in.read(buf)) > 0) {
                    out.write(buf, 0, n);
                }
                session.fsync(out);
            }
            Intent cb = new Intent(this, MainActivity.class);
            cb.setAction("com.yyoo.vitagallery.INSTALL_STATUS");
            cb.addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
            PendingIntent piCb = PendingIntent.getActivity(this, id, cb,
                    PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_MUTABLE);
            session.commit(piCb.getIntentSender());
            session.close();
            session = null;
        } catch (Exception e) {
            if (session != null) {
                try {
                    session.abandon();
                } catch (Exception ignored) {
                }
            }
            toast("Install failed: " + e.getMessage());
        }
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        handleInstallerStatus(intent);
    }

    // A committed PackageInstaller session shows no UI by itself: the
    // status intent arrives here, and on STATUS_PENDING_USER_ACTION
    // the attached confirmation intent must be started.
    private void handleInstallerStatus(Intent intent) {
        if (intent == null) return;
        int status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, -2);
        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            Intent confirm = intent.getParcelableExtra(Intent.EXTRA_INTENT);
            if (confirm != null) {
                confirm.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                startActivity(confirm);
            }
        } else if (status != -2 && status != PackageInstaller.STATUS_SUCCESS) {
            String msg = intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE);
            toast("Install: " + (msg == null ? "status " + status : msg));
        }
    }

    @Override
    protected void onDestroy() {
        destroying = true;
        if (isFinishing() && server != null) {
            server.destroy();
        }
        super.onDestroy();
    }
}
