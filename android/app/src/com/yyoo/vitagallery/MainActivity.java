package com.yyoo.vitagallery;

import android.app.Activity;
import android.app.DownloadManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.view.WindowManager;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Toast;

import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.URL;
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
        web.setDownloadListener((url, ua, disp, mime, len) -> startDownload(url, disp, mime));
        web.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageFinished(WebView view, String url) {
                loaded = true;
            }
        });
        setContentView(web);
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

    @Override
    protected void onDestroy() {
        destroying = true;
        if (isFinishing() && server != null) {
            server.destroy();
        }
        super.onDestroy();
    }
}
