// Vita Gallery 画面監査: jsdom で実際の操作経路を通す (多言語含む)
const fs = require("fs");
const path = require("path");
const { JSDOM } = require(path.join(process.env.HOME, "workspace/toolchain/jsdomlib/node_modules/jsdom"));

const html = fs.readFileSync(path.join(__dirname, "..", "page.html"), "utf8");

const LIST = {
  items: [
    { path: "root.jpg", folder: "", name: "root.jpg", size: 1000, ext: "jpg" },
    { path: "GameA/shot1.jpg", folder: "GameA", name: "shot1.jpg", size: 2000, ext: "jpg" },
    { path: "GameA/shot2.bmp", folder: "GameA", name: "shot2.bmp", size: 3000, ext: "bmp" },
    { path: "GameB/pic.bmp", folder: "GameB", name: "pic.bmp", size: 4000, ext: "bmp" },
  ],
  count: 4, folders: 3, root: "/ux0:/picture/SCREENSHOT",
};

let fail = 0;
function assert(cond, msg) {
  if (cond) console.log("ok -", msg);
  else { fail++; console.log("FAIL -", msg); }
}
const tick = (ms) => new Promise(r => setTimeout(r, ms || 30));

function makeDom(navLang) {
  return new JSDOM(html, {
    url: "http://127.0.0.1:47660/",
    runScripts: "dangerously",
    pretendToBeVisual: true,
    beforeParse(window) {
      if (navLang) {
        Object.defineProperty(window.navigator, "language", { value: navLang, configurable: true });
      }
      window.URL.createObjectURL = () => "blob:fake";
      window.URL.revokeObjectURL = () => {};
      window.fetch = (url, opts) => {
        const u = String(url);
        if (u.startsWith("/api/list")) {
          return Promise.resolve({ ok: true, json: () => Promise.resolve(JSON.parse(JSON.stringify(LIST))) });
        }
        if (u.startsWith("/api/delete-many")) {
          const body = JSON.parse(opts.body);
          return Promise.resolve({ ok: true, json: () => Promise.resolve({ deleted: body.paths.length, failed: [] }) });
        }
        if (u.startsWith("/api/dlstatus")) {
          return Promise.resolve({ ok: true, json: () => Promise.resolve({ state: "done" }) });
        }
        return Promise.resolve({ ok: true, json: () => Promise.resolve({}) });
      };
    },
  });
}

(async () => {
  // ---- 1) システム言語が既定になる (en-US → 英語, fr-FR → フランス語) ----
  {
    const dom = makeDom("en-US");
    const d = dom.window.document;
    await tick();
    assert(d.getElementById("btn-load").textContent === "Load", "default lang en-US -> Load button, got " + d.getElementById("btn-load").textContent);
    assert(dom.window.LANG === "en", "LANG=en for en-US");
    dom.window.close();
  }
  {
    const dom = makeDom("fr-FR");
    const d = dom.window.document;
    await tick();
    assert(d.getElementById("btn-load").textContent === "Charger", "default lang fr-FR -> Charger, got " + d.getElementById("btn-load").textContent);
    dom.window.close();
  }
  {
    const dom = makeDom("ja-JP");
    const d = dom.window.document;
    await tick();
    assert(d.getElementById("btn-load").textContent === "読み込む", "default lang ja-JP -> 読み込む");
    dom.window.close();
  }
  {
    const dom = makeDom("zh-TW");
    const d = dom.window.document;
    await tick();
    assert(dom.window.LANG === "zh-Hant", "zh-TW -> zh-Hant, got " + dom.window.LANG);
    dom.window.close();
  }

  // ---- 2) メイン監査 (日本語に切り替えてから) ----
  const dom = makeDom("en-US");
  const { window } = dom;
  const d = window.document;
  await tick();

  // 全言語で全キーが揃い、t() がキー名をそのまま返さない
  const langs = window.LANGS;
  const keys = Object.keys(window.I18N.en);
  assert(langs.length === 12, "12 languages, got " + langs.length);
  for (const lg of langs) {
    const dict = window.I18N[lg];
    const missing = keys.filter(k => !(k in dict) || dict[k] === "");
    assert(missing.length === 0, "lang " + lg + " has all " + keys.length + " keys" + (missing.length ? " missing: " + missing.join(",") : ""));
  }
  // 言語切替で表示が変わる
  const sel0 = d.getElementById("lang-sel");
  for (const lg of ["de", "ko", "zh-Hans", "ja"]) {
    sel0.value = lg;
    sel0.dispatchEvent(new window.Event("change"));
    await tick(5);
    assert(d.getElementById("btn-load").textContent === window.I18N[lg].load, "switch to " + lg + " updates Load button");
    assert(d.getElementById("btn-quit").textContent === window.I18N[lg].quit, "switch to " + lg + " updates Quit button");
    assert(window.t("errConnect") === window.I18N[lg].errConnect && window.t("errConnect") !== "errConnect", "t(errConnect) resolves in " + lg);
  }
  // ja に戻っている状態でメイン経路
  assert(d.getElementById("btn-load").textContent === "読み込む", "back to ja");

  assert(!d.getElementById("page-home").classList.contains("hidden"), "home visible at start");
  assert(d.getElementById("page-gallery").classList.contains("hidden"), "gallery hidden at start");

  d.getElementById("ip").value = "192.168.1.10";
  d.getElementById("btn-load").click();
  await tick(); await tick();
  assert(!d.getElementById("page-gallery").classList.contains("hidden"), "gallery visible after load");
  const titles = [...d.querySelectorAll(".folder-title")].map(e => e.textContent);
  assert(titles.some(t => t.includes("SCREENSHOT 直下")), "root folder group shown");
  assert(titles.some(t => t.includes("GameA")), "GameA group shown");
  assert(d.querySelectorAll(".item").length === 4, "4 cards rendered");
  assert(d.getElementById("gal-summary").textContent.includes("4"), "summary shows count");

  const sel = d.getElementById("folder-filter");
  sel.value = "GameA";
  sel.dispatchEvent(new window.Event("change"));
  await tick();
  assert(d.querySelectorAll(".item").length === 2, "filter GameA shows 2");
  sel.selectedIndex = 0;
  sel.dispatchEvent(new window.Event("change"));
  await tick();
  assert(d.querySelectorAll(".item").length === 4, "filter reset shows 4");

  // 表示モーダル
  d.querySelector(".item .thumb").click();
  await tick();
  assert(d.getElementById("viewer-bg").classList.contains("open"), "viewer opens");
  assert(d.getElementById("viewer-info").textContent.includes("root.jpg"), "viewer info shows name");
  d.getElementById("btn-view-close").click();
  assert(!d.getElementById("viewer-bg").classList.contains("open"), "viewer closes");

  // ダウンロード: 「ダウンロード中…」が出て、完了で結果ダイアログに切り替わる
  d.querySelector('.item [data-op="dl"]').click();
  await tick();
  assert(d.getElementById("dl-bg").classList.contains("open"), "download-in-progress dialog opens");
  assert(d.getElementById("dl-msg").textContent.includes("ダウンロードしています"), "dl dialog text (ja)");
  await tick(900); // ポーリング間隔 700ms
  assert(!d.getElementById("dl-bg").classList.contains("open"), "dl dialog closes when done");
  assert(d.getElementById("dlg-bg").classList.contains("open") && d.getElementById("dlg-msg").textContent.includes("ダウンロードしました"), "result dialog shows downloaded");
  d.getElementById("dlg-ok").click();
  await tick();

  // 選択 → ZIP ダウンロードも同じ表示経路
  const cb = d.querySelector(".item input[type=checkbox]");
  cb.checked = true;
  cb.dispatchEvent(new window.Event("change"));
  await tick();
  assert(d.getElementById("sel-count").textContent.includes("1"), "sel count 1");
  assert(!d.getElementById("btn-zip").disabled, "zip button enabled");
  d.getElementById("btn-zip").click();
  await tick();
  assert(d.getElementById("dl-bg").classList.contains("open"), "zip download shows progress dialog");
  await tick(900);
  assert(d.getElementById("dlg-msg").textContent.includes("ダウンロードしました"), "zip result dialog shows downloaded");
  d.getElementById("dlg-ok").click();
  await tick();

  // 選択削除
  d.getElementById("btn-del-sel").click();
  await tick();
  assert(d.getElementById("dlg-bg").classList.contains("open"), "delete confirm dialog opens");
  assert(d.getElementById("dlg-msg").textContent.includes("削除"), "confirm dialog mentions delete");
  d.getElementById("dlg-ok").click();
  await tick(); await tick();
  assert(d.querySelectorAll(".item").length === 3, "1 item removed after delete");
  assert(d.getElementById("dlg-msg").textContent.includes("削除しました"), "result dialog shows deleted");
  d.getElementById("dlg-ok").click();

  // Android 戻るボタン用フック: ギャラリーではホームへ戻り true、ホームでは false
  assert(window.__vitaBack() === true, "__vitaBack true on gallery");
  assert(!d.getElementById("page-home").classList.contains("hidden"), "__vitaBack went home");
  assert(window.__vitaBack() === false, "__vitaBack false on home");

  console.log(fail === 0 ? "AUDIT_ALL_PASS" : "AUDIT_FAILURES=" + fail);
  process.exit(fail === 0 ? 0 : 1);
})().catch(e => { console.error("AUDIT_ERROR", e); process.exit(1); });
