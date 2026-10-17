"use strict";

const STORAGE_KEY = "weicopy.session";
const PRESETS = [
  [10, "10 分钟"],
  [30, "30 分钟"],
  [60, "1 小时"],
  [360, "6 小时"],
  [1440, "1 天"],
  [10080, "7 天"],
  [0, "永久"],
];

let current = null;
let allItems = [];
let itemsSig = null;
let editingId = null;
let pendingItems = null;
let filter = "all";
let pollTimer = null;
let clockTimer = null;
let toastTimer = null;
let busy = false;
let uploading = false;
let switching = false;
let dragDepth = 0;
const objectURLs = new Map();

function loadLocal() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const data = JSON.parse(raw);
    if (!data || typeof data.name !== "string" || !data.name.trim()) return null;
    const ttl = Number(data.ttlMinutes);
    const hasPassword = Object.prototype.hasOwnProperty.call(data, "password");
    return {
      name: data.name.trim(),
      password: hasPassword ? String(data.password) : "",
      ttlMinutes: Number.isFinite(ttl) ? ttl : 30,
      paused: data.paused === true || !hasPassword,
    };
  } catch (err) {
    return null;
  }
}

function remember(session, password) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({
    name: session.name,
    password: password || "",
    ttlMinutes: session.ttl_minutes,
    paused: false,
  }));
}

function sessionURL(name, suffix) {
  return `/api/sessions/${encodeURIComponent(name)}${suffix || ""}`;
}

function headerValue(password) {
  const bytes = new TextEncoder().encode(password);
  let value = "";
  for (const byte of bytes) value += String.fromCharCode(byte);
  return value;
}

async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (options.password) headers.set("Authorization", headerValue(options.password));
  let body;
  if (options.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(options.json);
  } else if (options.raw !== undefined) {
    headers.set("Content-Type", "text/plain; charset=utf-8");
    body = options.raw;
  } else if (options.form) {
    body = options.form;
  }

  let response;
  try {
    response = await fetch(path, { method: options.method || "GET", headers, body });
  } catch (err) {
    const error = new Error("无法连接服务器");
    error.status = 0;
    throw error;
  }

  const type = response.headers.get("content-type") || "";
  let data = null;
  if (type.includes("application/json")) {
    data = await response.json();
  } else if (!response.ok) {
    data = { message: await response.text() };
  }
  if (!response.ok) {
    const error = new Error((data && data.message) || "请求失败");
    error.status = response.status;
    error.code = data && data.error;
    throw error;
  }
  return data;
}

async function fetchBlob(path) {
  const headers = {};
  if (current.password) headers.Authorization = headerValue(current.password);
  const response = await fetch(path, { headers });
  if (!response.ok) {
    let message = "下载失败";
    try {
      const data = await response.json();
      if (data && data.message) message = data.message;
    } catch (err) {
      /* ignore non-json errors */
    }
    const error = new Error(message);
    error.status = response.status;
    throw error;
  }
  return response.blob();
}

function showToast(message, kind) {
  const el = document.getElementById("toast");
  el.hidden = false;
  el.textContent = message;
  el.className = "toast" + (kind === "error" ? " toast-error" : "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    el.hidden = true;
  }, 2800);
}

function showFormError(text) {
  const el = document.getElementById("form-error");
  if (!el) return;
  if (!text) {
    el.hidden = true;
    el.textContent = "";
    return;
  }
  el.hidden = false;
  el.className = "alert alert-error";
  el.setAttribute("role", "alert");
  el.textContent = text;
}

function applyMessage(message, kind) {
  const el = document.getElementById("form-error");
  if (!el) return;
  if (!message) {
    showFormError("");
    return;
  }
  if (kind === "error") {
    showFormError(message);
    return;
  }
  el.hidden = false;
  el.className = "alert alert-info";
  el.textContent = message;
}

async function withBusy(button, label, fn) {
  if (busy) return;
  busy = true;
  const previous = button.textContent;
  button.disabled = true;
  button.textContent = label;
  try {
    await fn();
  } finally {
    busy = false;
    if (button.isConnected) {
      button.disabled = false;
      button.textContent = previous;
    }
  }
}

function teardownRoom() {
  clearInterval(pollTimer);
  clearInterval(clockTimer);
  pollTimer = null;
  clockTimer = null;
  for (const url of objectURLs.values()) URL.revokeObjectURL(url);
  objectURLs.clear();
  itemsSig = null;
  editingId = null;
  pendingItems = null;
  allItems = [];
  current = null;
  filter = "all";
}

function describeTTL(minutes) {
  if (minutes === 0) return "永久";
  if (minutes % 1440 === 0 && minutes >= 1440) return `${minutes / 1440} 天`;
  if (minutes % 60 === 0 && minutes >= 60) return `${minutes / 60} 小时`;
  return `${minutes} 分钟`;
}

function formatClock(iso) {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatRemaining(session) {
  if (!session || session.permanent || !session.expires_at) return "永久保留";
  const ms = new Date(session.expires_at).getTime() - Date.now();
  if (ms <= 0) return "即将结束";
  const min = Math.ceil(ms / 60000);
  if (min < 60) return `剩余 ${min} 分钟`;
  const hours = Math.floor(min / 60);
  const minutes = min % 60;
  if (hours < 48) return minutes ? `剩余 ${hours} 小时 ${minutes} 分钟` : `剩余 ${hours} 小时`;
  const days = Math.floor(hours / 24);
  const rest = hours % 24;
  return rest ? `剩余 ${days} 天 ${rest} 小时` : `剩余 ${days} 天`;
}

function formatSize(bytes) {
  const n = Number(bytes) || 0;
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(n < 10 * 1024 ? 1 : 0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function signature(items) {
  return (items || []).map((item) => `${item.id}:${item.updated_at}:${item.size}`).join("|");
}

function wasEdited(item) {
  return new Date(item.updated_at).getTime() - new Date(item.created_at).getTime() > 0;
}

function readTTL() {
  const input = document.getElementById("ttl-input");
  if (!input) return 30;
  const raw = input.value.trim();
  if (!/^\d+$/.test(raw)) return NaN;
  return Number(raw);
}

function syncChips() {
  const value = readTTL();
  document.querySelectorAll("#ttl-chips .chip").forEach((chip) => {
    chip.setAttribute("aria-pressed", String(Number(chip.dataset.value) === value));
  });
}

function updateTTLHelp() {
  const el = document.getElementById("ttl-help");
  if (!el) return;
  const minutes = readTTL();
  if (Number.isNaN(minutes)) {
    el.textContent = "请输入 0 到 525600 之间的整数，0 表示永久";
    return;
  }
  if (minutes === 0) {
    el.textContent = "永久保留，直到有人在会话里手动结束";
    return;
  }
  el.textContent = `从创建时起 ${describeTTL(minutes)}后自动清除，中途进入不会顺延`;
}

function setupTTL(currentTTL) {
  const wrap = document.getElementById("ttl-chips");
  wrap.replaceChildren();
  for (const [value, label] of PRESETS) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "chip";
    button.dataset.value = String(value);
    button.textContent = label;
    button.setAttribute("aria-pressed", String(value === Number(currentTTL)));
    button.addEventListener("click", () => {
      document.getElementById("ttl-input").value = String(value);
      syncChips();
      updateTTLHelp();
    });
    wrap.appendChild(button);
  }
  const input = document.getElementById("ttl-input");
  input.value = String(currentTTL);
  input.addEventListener("input", () => {
    syncChips();
    updateTTLHelp();
  });
  updateTTLHelp();
}

function showGate(options = {}) {
  teardownRoom();
  const mode = options.mode || "enter";
  document.body.dataset.view = "gate";
  document.title = "WeiCopy";
  const ttl = options.ttl === undefined || options.ttl === null ? 30 : options.ttl;
  const app = document.getElementById("app");
  app.innerHTML = `
    <main class="shell">
      <section class="card gate-card">
        <div class="brand"><span class="brand-mark">W</span><span class="brand-name">WeiCopy</span></div>
        <h1 id="gate-title"></h1>
        <p class="subtitle" id="gate-sub"></p>
        <form id="gate-form" class="stack">
          <div id="form-error" class="alert" hidden></div>
          <div class="field">
            <label for="session-name">会话名称</label>
            <input id="session-name" name="weicopy-session" maxlength="40" autocomplete="off" spellcheck="false" placeholder="例如 weicopy">
          </div>
          <p class="help" id="join-hint" hidden></p>
          <div class="field" id="ttl-field">
            <label for="ttl-input">持续时长</label>
            <div class="chips" id="ttl-chips"></div>
            <div class="row">
              <input id="ttl-input" inputmode="numeric" aria-describedby="ttl-help" autocomplete="off">
              <span class="help">分钟，0 为永久</span>
            </div>
            <p class="help" id="ttl-help"></p>
            <p class="help" id="ttl-note"></p>
          </div>
          <div class="field" id="password-field">
            <label for="session-password" id="password-label">密码（可选）</label>
            <input id="session-password" name="weicopy-session-password" type="password" maxlength="72" autocomplete="off" placeholder="留空则不设密码">
          </div>
          <button id="submit-btn" class="btn btn-primary btn-block" type="submit"></button>
          <button id="switch-btn" class="btn btn-ghost btn-block" type="button" hidden>换一个会话</button>
        </form>
      </section>
    </main>`;

  const copy = {
    enter: ["进入一个会话", "选一个名字。另一台设备进入同一个名字，就能看到、修改和删除里面的内容。"],
    join: ["继续进入", "这个会话还在有效期内。"],
    reinit: ["重新初始化", "沿用原来的名字，重新设定时长和密码。确认后是一个空会话。"],
  };
  document.getElementById("gate-title").textContent = copy[mode][0];
  document.getElementById("gate-sub").textContent = copy[mode][1];
  document.getElementById("session-name").value = options.name || "";
  if (mode !== "enter") document.getElementById("session-name").readOnly = true;
  document.getElementById("submit-btn").textContent = mode === "reinit" ? "重新初始化" : "进入会话";
  document.getElementById("password-label").textContent = mode === "join" ? "密码" : "密码（可选）";
  document.getElementById("session-password").placeholder = mode === "join" ? "输入会话密码" : "留空则不设密码";
  document.getElementById("ttl-note").textContent = mode === "reinit"
    ? "确认后会清空这个名字下的旧内容，并重新开始计时。"
    : "若这个名字已有会话，会直接进入，时长和密码都以当时创建的为准。";

  if (mode === "join") {
    document.getElementById("ttl-field").hidden = true;
    const hint = document.getElementById("join-hint");
    hint.hidden = false;
    hint.textContent = options.hasPassword ? "这个会话设了密码。" : "这个会话没有密码，直接进入即可。";
    if (!options.hasPassword) document.getElementById("password-field").hidden = true;
  } else {
    setupTTL(ttl);
  }

  const switchBtn = document.getElementById("switch-btn");
  if (mode === "join" || mode === "reinit") {
    switchBtn.hidden = false;
    switchBtn.textContent = mode === "join" ? "换一个会话" : "改为开始新的";
    switchBtn.addEventListener("click", startFresh);
  }

  applyMessage(options.message || "", options.messageKind || "info");
  const form = document.getElementById("gate-form");
  form.addEventListener("input", () => showFormError(""));
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const button = document.getElementById("submit-btn");
    if (mode === "reinit") submitReinit(button);
    else if (mode === "join") submitJoin(button);
    else submitEnter(button);
  });
  if (mode === "enter") document.getElementById("session-name").focus();
  else if (mode === "join" && options.hasPassword) document.getElementById("session-password").focus();
}

function readGate() {
  return {
    name: document.getElementById("session-name").value.trim(),
    password: document.getElementById("session-password") ? document.getElementById("session-password").value : "",
    ttl: readTTL(),
  };
}

function validateGate(fields, checkTTL) {
  if (!fields.name) {
    showFormError("请填写会话名称");
    return false;
  }
  if ([...fields.name].length > 40) {
    showFormError("会话名称最长 40 个字符");
    return false;
  }
  if (/[\\/?#%]/.test(fields.name)) {
    showFormError("会话名称不能包含 / \\ ? # %");
    return false;
  }
  if (new TextEncoder().encode(fields.password).length > 72) {
    showFormError("密码最长 72 字节");
    return false;
  }
  if (checkTTL && (Number.isNaN(fields.ttl) || fields.ttl > 525600)) {
    showFormError("时长需要是 0 到 525600 之间的整数，0 表示永久");
    return false;
  }
  showFormError("");
  return true;
}

async function submitEnter(button) {
  const fields = readGate();
  if (!validateGate(fields, true)) return;
  await withBusy(button, "正在进入…", async () => {
    try {
      const status = await api("/api/sessions/status?name=" + encodeURIComponent(fields.name));
      if (status.exists) {
        if (status.has_password && fields.password === "") {
          showFormError("这个会话已经存在，请输入密码后再进入");
          return;
        }
        if (!status.has_password && fields.password !== "") {
          showFormError("这个会话没有密码。请清空密码后再进入，避免误以为内容受密码保护");
          return;
        }
        await openSession(fields.name, fields.password);
        showToast("已进入现有会话");
        return;
      }
      const data = await createSession(fields);
      showToast(data.session.permanent ? "已创建，会一直保留到手动结束" : `已创建，${describeTTL(data.session.ttl_minutes)}后过期`);
      await showRoom(fields.password, data.session, []);
    } catch (err) {
      if (err.status === 409) {
        try {
          await openSession(fields.name, fields.password);
          showToast("已进入现有会话");
          return;
        } catch (joinErr) {
          showFormError(joinErr.message);
          return;
        }
      }
      showFormError(err.message);
    }
  });
}

async function submitJoin(button) {
  const fields = readGate();
  if (!validateGate(fields, false)) return;
  await withBusy(button, "正在进入…", async () => {
    try {
      await openSession(fields.name, fields.password);
    } catch (err) {
      showFormError(err.message);
    }
  });
}

async function submitReinit(button) {
  const fields = readGate();
  if (!validateGate(fields, true)) return;
  await withBusy(button, "正在初始化…", async () => {
    try {
      const data = await createSession(fields);
      showToast("已重新初始化");
      await showRoom(fields.password, data.session, []);
    } catch (err) {
      if (err.status === 409) {
        showFormError("这个名字刚被占用了，可以换一个，或进入现有会话");
        return;
      }
      showFormError(err.message);
    }
  });
}

function createSession(fields) {
  return api("/api/sessions", {
    method: "POST",
    json: { name: fields.name, password: fields.password, ttl_minutes: fields.ttl },
  });
}

async function openSession(name, password) {
  const data = await api(sessionURL(name, "/state"), { password });
  await showRoom(password, data.session, data.items);
}

function showExpired(stored) {
  teardownRoom();
  document.body.dataset.view = "expired";
  document.title = "WeiCopy";
  const app = document.getElementById("app");
  app.innerHTML = `
    <main class="shell">
      <section class="card gate-card">
        <div class="brand"><span class="brand-mark">W</span><span class="brand-name">WeiCopy</span></div>
        <h1>会话已结束</h1>
        <p class="subtitle">浏览器还记着上次的会话，但服务器上已经超时，内容也清掉了。</p>
        <p class="name-banner" id="expired-name"></p>
        <div class="stack">
          <button type="button" class="btn btn-primary btn-block" id="reinit-btn">重新初始化</button>
          <button type="button" class="btn btn-ghost btn-block" id="fresh-btn">开始新的</button>
        </div>
      </section>
    </main>`;
  document.getElementById("expired-name").textContent = stored.name;
  document.getElementById("reinit-btn").addEventListener("click", () => {
    showGate({ mode: "reinit", name: stored.name, ttl: stored.ttlMinutes ?? 30 });
  });
  document.getElementById("fresh-btn").addEventListener("click", startFresh);
  document.getElementById("reinit-btn").focus();
}

function startFresh() {
  localStorage.removeItem(STORAGE_KEY);
  history.replaceState(null, "", "/");
  showGate({ mode: "enter", ttl: 30 });
}

function handleRoomError(err) {
  if (switching) return;
  if (!err || (err.status !== 401 && err.status !== 410)) {
    showToast((err && err.message) || "操作失败", "error");
    return;
  }
  switching = true;
  const stored = loadLocal();
  const name = (stored && stored.name) || (current && current.session && current.session.name) || "";
  const ttl = stored && Number.isFinite(stored.ttlMinutes) ? stored.ttlMinutes : 30;
  teardownRoom();
  const finish = () => {
    switching = false;
  };
  if (!name) {
    showGate({ mode: "enter", message: err.message, messageKind: "error" });
    finish();
    return;
  }
  if (err.status === 410) {
    showExpired({ name, ttlMinutes: ttl });
    finish();
    return;
  }
  api("/api/sessions/status?name=" + encodeURIComponent(name))
    .then((status) => {
      if (!status.exists) showExpired({ name, ttlMinutes: ttl });
      else {
        showGate({
          mode: "join",
          name,
          hasPassword: status.has_password,
          ttl: status.ttl_minutes,
          message: err.message,
          messageKind: "error",
        });
      }
    })
    .catch((statusErr) => {
      showGate({ mode: "enter", name, message: statusErr.message, messageKind: "error" });
    })
    .finally(finish);
}

function updateMeta() {
  const el = document.getElementById("session-meta");
  if (!el || !current) return;
  const session = current.session;
  const pass = session.has_password ? "有密码" : "无密码";
  if (session.permanent || !session.expires_at) {
    el.textContent = `永久保留 · ${pass}`;
    return;
  }
  const when = formatClock(session.expires_at);
  el.textContent = when
    ? `${formatRemaining(session)} · ${pass} · ${when} 结束`
    : `${formatRemaining(session)} · ${pass}`;
}

function updateCount() {
  const el = document.getElementById("count-label");
  if (el) el.textContent = `${allItems.length} 条`;
}

function setupFilters() {
  const wrap = document.getElementById("filters");
  wrap.replaceChildren();
  for (const [value, label] of [["all", "全部"], ["text", "文本"], ["image", "图片"], ["file", "文件"]]) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "chip";
    button.dataset.value = value;
    button.textContent = label;
    button.setAttribute("aria-pressed", String(filter === value));
    button.addEventListener("click", () => {
      filter = value;
      for (const chip of wrap.children) {
        chip.setAttribute("aria-pressed", String(chip.dataset.value === filter));
      }
      renderItems(allItems);
    });
    wrap.appendChild(button);
  }
}

function buildRoomShell() {
  const app = document.getElementById("app");
  app.innerHTML = `
    <main class="shell">
      <header class="top">
        <div>
          <p class="kicker">WeiCopy</p>
          <h1 class="session-title" id="session-name"></h1>
          <p class="meta" id="session-meta"></p>
        </div>
        <div class="actions">
          <button type="button" class="btn btn-ghost" id="copy-link">复制链接</button>
          <button type="button" class="btn btn-ghost" id="leave-btn">离开</button>
          <button type="button" class="btn btn-danger" id="end-btn">结束会话</button>
        </div>
      </header>
      <section class="card composer">
        <div class="field">
          <label for="text-input">添加文本</label>
          <textarea id="text-input" rows="4" placeholder="输入或粘贴文本"></textarea>
        </div>
        <div class="row">
          <button type="button" class="btn btn-primary" id="add-text">添加文本</button>
          <button type="button" class="btn" id="pick-file">上传文件</button>
          <input id="file-input" type="file" multiple hidden>
          <span class="help">可直接粘贴图片，或把文件拖进来。Ctrl/⌘ + Enter 添加文本</span>
        </div>
        <p class="help" id="upload-status" hidden></p>
      </section>
      <div class="row spread">
        <h2>内容 <span class="muted" id="count-label"></span></h2>
        <div class="chips" id="filters"></div>
      </div>
      <div id="list"></div>
    </main>`;
  document.getElementById("session-name").textContent = current.session.name;
  document.getElementById("copy-link").addEventListener("click", copyLink);
  document.getElementById("leave-btn").title = "只断开这台设备，会话和内容都还在";
  document.getElementById("leave-btn").addEventListener("click", leave);
  document.getElementById("end-btn").title = "清空内容并让这个会话立刻失效";
  document.getElementById("end-btn").addEventListener("click", endSession);
  document.getElementById("add-text").addEventListener("click", addText);
  document.getElementById("text-input").addEventListener("keydown", (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      addText();
    }
  });
  const fileInput = document.getElementById("file-input");
  document.getElementById("pick-file").addEventListener("click", () => fileInput.click());
  fileInput.addEventListener("change", () => {
    const files = [...fileInput.files];
    fileInput.value = "";
    uploadFiles(files);
  });
  updateMeta();
  setupFilters();
}

async function showRoom(password, session, items) {
  teardownRoom();
  current = { password: password || "", session };
  remember(session, current.password);
  filter = "all";
  document.body.dataset.view = "room";
  document.title = `WeiCopy · ${session.name}`;
  history.replaceState(null, "", "/?s=" + encodeURIComponent(session.name));
  buildRoomShell();
  try {
    if (items) renderItems(items);
    else await refresh();
  } catch (err) {
    handleRoomError(err);
    return;
  }
  startPoll();
  startClock();
  const input = document.getElementById("text-input");
  if (input) input.focus();
}

function startPoll() {
  clearInterval(pollTimer);
  pollTimer = setInterval(() => {
    if (document.hidden || !current) return;
    refresh({ keepEditing: true }).catch((err) => {
      if (err.status === 401 || err.status === 410) handleRoomError(err);
    });
  }, 3000);
}

function startClock() {
  clearInterval(clockTimer);
  clockTimer = setInterval(updateMeta, 15000);
}

async function refresh(options = {}) {
  if (!current) return;
  const state = await api(sessionURL(current.session.name, "/state"), { password: current.password });
  if (!current) return;
  current.session = state.session;
  updateMeta();
  const items = state.items || [];
  if (options.keepEditing && editingId) {
    pendingItems = items;
    return;
  }
  if (signature(items) === itemsSig && document.getElementById("list")) return;
  renderItems(items);
}

function renderItems(items) {
  allItems = items || [];
  itemsSig = signature(allItems);
  const list = document.getElementById("list");
  if (!list) return;
  const live = new Set(allItems.filter((item) => item.type === "image").map((item) => item.id));
  for (const [id, url] of objectURLs) {
    if (!live.has(id)) {
      URL.revokeObjectURL(url);
      objectURLs.delete(id);
    }
  }
  const shown = allItems.filter((item) => filter === "all" || item.type === filter);
  list.replaceChildren();
  updateCount();
  if (!shown.length) {
    const empty = document.createElement("div");
    empty.className = "card empty";
    empty.textContent = allItems.length
      ? "这个分类下还没有内容"
      : "还没有内容。贴上文本或文件后，另一台设备进入同一会话就能看到。";
    list.appendChild(empty);
    return;
  }
  for (const item of shown) list.appendChild(renderItem(item));
}

function renderItem(item) {
  const card = document.createElement("article");
  card.className = "card item";
  const head = document.createElement("div");
  head.className = "item-head";
  const left = document.createElement("div");
  left.className = "item-idline";
  const pill = document.createElement("span");
  pill.className = "pill";
  pill.textContent = { text: "文本", image: "图片", file: "文件" }[item.type] || item.type;
  const time = document.createElement("span");
  time.className = "muted";
  time.textContent = wasEdited(item) ? `${formatClock(item.updated_at)} 更新` : formatClock(item.created_at);
  left.append(pill, time);

  const actions = document.createElement("div");
  actions.className = "actions";
  if (item.type === "text" && editingId !== item.id) {
    actions.append(
      smallBtn("复制", () => copyText(item.content, "已复制")),
      smallBtn("更新", () => startEdit(item)),
    );
  }
  if (item.type === "file") actions.append(smallBtn("下载", () => downloadItem(item).catch(onItemErr)));
  if (item.type === "image") actions.append(smallBtn("打开", () => openImage(item).catch(onItemErr)));
  if (editingId !== item.id) actions.append(smallBtn("删除", () => deleteItem(item), "btn-danger"));
  head.append(left, actions);
  card.append(head);

  if (item.type === "text" && editingId === item.id) {
    const area = document.createElement("textarea");
    area.id = "edit-text";
    area.value = item.content;
    const row = document.createElement("div");
    row.className = "row";
    row.append(
      smallBtn("保存", () => saveEdit(item), "btn-primary"),
      smallBtn("取消", () => cancelEdit()),
    );
    card.append(area, row);
  } else if (item.type === "text") {
    const pre = document.createElement("pre");
    pre.className = "content";
    pre.textContent = item.content;
    card.append(pre);
  } else if (item.type === "image") {
    const img = document.createElement("img");
    img.className = "preview";
    img.alt = item.filename || "图片";
    const caption = document.createElement("p");
    caption.className = "muted file-line";
    caption.textContent = fileLine(item);
    card.append(img, caption);
    loadPreview(item, img);
  } else {
    const line = document.createElement("p");
    line.className = "file-line";
    line.textContent = fileLine(item);
    card.append(line);
  }
  return card;
}

function smallBtn(label, onClick, extra) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "btn btn-small" + (extra ? " " + extra : "");
  button.textContent = label;
  button.addEventListener("click", onClick);
  return button;
}

function fileLine(item) {
  return `${item.filename || "未命名"} · ${formatSize(item.size)}`;
}

function startEdit(item) {
  editingId = item.id;
  renderItems(allItems);
  const area = document.getElementById("edit-text");
  if (area) {
    area.focus();
    area.setSelectionRange(area.value.length, area.value.length);
  }
}

function cancelEdit() {
  editingId = null;
  if (pendingItems) {
    const items = pendingItems;
    pendingItems = null;
    renderItems(items);
    return;
  }
  renderItems(allItems);
}

async function saveEdit(item) {
  const area = document.getElementById("edit-text");
  const value = area ? area.value : "";
  if (!value) {
    showToast("内容不能是空的", "error");
    return;
  }
  try {
    await api(sessionURL(current.session.name, `/items/${item.id}`), {
      method: "PUT",
      password: current.password,
      json: { content: value },
    });
    editingId = null;
    pendingItems = null;
    showToast("已更新");
    await refresh();
  } catch (err) {
    if (err.status === 401 || err.status === 410) handleRoomError(err);
    else showToast(err.message, "error");
  }
}

async function deleteItem(item) {
  try {
    await api(sessionURL(current.session.name, `/items/${item.id}`), { method: "DELETE", password: current.password });
    if (editingId === item.id) editingId = null;
    showToast("已删除");
    await refresh();
  } catch (err) {
    if (err.status === 401 || err.status === 410) handleRoomError(err);
    else showToast(err.message, "error");
  }
}

async function addText() {
  if (!current || busy) return;
  const input = document.getElementById("text-input");
  const text = input.value;
  if (!text) {
    showToast("文本是空的", "error");
    return;
  }
  const button = document.getElementById("add-text");
  await withBusy(button, "正在添加…", async () => {
    try {
      await api(sessionURL(current.session.name, "/items/text"), { method: "POST", password: current.password, raw: text });
      input.value = "";
      showToast("已添加");
      await refresh();
    } catch (err) {
      if (err.status === 401 || err.status === 410) handleRoomError(err);
      else showToast(err.message, "error");
    }
  });
}

async function uploadFiles(fileList) {
  if (uploading || !current) return;
  const files = [...fileList].filter(Boolean);
  if (!files.length) return;
  uploading = true;
  const status = document.getElementById("upload-status");
  if (status) status.hidden = false;
  try {
    for (let i = 0; i < files.length; i += 1) {
      if (status) status.textContent = files.length > 1 ? `正在上传 ${i + 1}/${files.length}…` : "正在上传…";
      const form = new FormData();
      form.append("file", files[i], files[i].name || "file");
      await api(sessionURL(current.session.name, "/items/file"), { method: "POST", password: current.password, form });
    }
    showToast(files.length > 1 ? `已上传 ${files.length} 个文件` : "已上传");
    await refresh();
  } catch (err) {
    if (err.status === 401 || err.status === 410) handleRoomError(err);
    else showToast(err.message || "上传失败", "error");
  } finally {
    uploading = false;
    if (status && status.isConnected) status.hidden = true;
  }
}

async function loadPreview(item, img) {
  if (objectURLs.has(item.id)) {
    img.src = objectURLs.get(item.id);
    return;
  }
  try {
    const blob = await fetchBlob(sessionURL(current.session.name, `/items/${item.id}/file`));
    if (!img.isConnected) return;
    const url = URL.createObjectURL(blob);
    objectURLs.set(item.id, url);
    img.src = url;
  } catch (err) {
    if (img.isConnected) img.alt = err.message || "图片加载失败";
  }
}

async function openImage(item) {
  if (!objectURLs.has(item.id)) {
    const blob = await fetchBlob(sessionURL(current.session.name, `/items/${item.id}/file`));
    objectURLs.set(item.id, URL.createObjectURL(blob));
  }
  window.open(objectURLs.get(item.id), "_blank", "noopener");
}

async function downloadItem(item) {
  const blob = await fetchBlob(sessionURL(current.session.name, `/items/${item.id}/file`));
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = item.filename || "download";
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1500);
}

function onItemErr(err) {
  if (err.status === 401 || err.status === 410) handleRoomError(err);
  else showToast(err.message || "操作失败", "error");
}

async function copyText(text, message) {
  try {
    await navigator.clipboard.writeText(text);
    showToast(message || "已复制");
  } catch (err) {
    showToast("复制失败", "error");
  }
}

function copyLink() {
  if (!current) return;
  copyText(`${location.origin}/?s=${encodeURIComponent(current.session.name)}`, "链接已复制");
}

function leave() {
  const stored = loadLocal();
  const name = (stored && stored.name) || (current && current.session && current.session.name) || "";
  const ttl = stored ? stored.ttlMinutes : 30;
  if (name) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ name, password: "", ttlMinutes: ttl, paused: true }));
  }
  teardownRoom();
  history.replaceState(null, "", name ? `/?s=${encodeURIComponent(name)}` : "/");
  if (!name) {
    showGate({ mode: "enter" });
    return;
  }
  api("/api/sessions/status?name=" + encodeURIComponent(name))
    .then((status) => {
      if (!status.exists) showExpired({ name, ttlMinutes: ttl });
      else showGate({ mode: "join", name, hasPassword: status.has_password, ttl: status.ttl_minutes });
    })
    .catch((err) => showGate({ mode: "enter", name, message: err.message, messageKind: "error" }));
}

async function endSession() {
  if (!current) return;
  if (!window.confirm("结束会话后，所有设备都会立刻失去这些内容。确定结束？")) return;
  try {
    await api(sessionURL(current.session.name, ""), { method: "DELETE", password: current.password });
  } catch (err) {
    if (err.status !== 410) {
      showToast(err.message, "error");
      return;
    }
  }
  localStorage.removeItem(STORAGE_KEY);
  teardownRoom();
  history.replaceState(null, "", "/");
  showToast("会话已结束");
  showGate({ mode: "enter" });
}

function hasFiles(event) {
  return [...(event.dataTransfer?.types || [])].includes("Files");
}

function onPaste(event) {
  if (document.body.dataset.view !== "room" || !current) return;
  const data = event.clipboardData;
  if (!data) return;
  const files = [];
  for (const item of data.items || []) {
    if (item.kind === "file" && item.type.startsWith("image/")) {
      const file = item.getAsFile();
      if (file) files.push(renamePasted(file));
    }
  }
  if (files.length) {
    event.preventDefault();
    uploadFiles(files);
    return;
  }
  const target = event.target;
  if (target && (target.tagName === "TEXTAREA" || target.tagName === "INPUT")) return;
  const text = data.getData("text/plain");
  if (!text) return;
  const input = document.getElementById("text-input");
  if (!input || editingId) return;
  event.preventDefault();
  input.value = input.value ? `${input.value}${input.value.endsWith("\n") ? "" : "\n"}${text}` : text;
  input.focus();
}

function renamePasted(file) {
  const ext = (file.type.split("/")[1] || "png").replace("jpeg", "jpg");
  const name = file.name && !/^image\./i.test(file.name) ? file.name : `pasted-${Date.now()}.${ext}`;
  try {
    return new File([file], name, { type: file.type });
  } catch (err) {
    return file;
  }
}

async function boot() {
  const params = new URLSearchParams(location.search);
  const queryName = (params.get("s") || "").trim();
  const stored = loadLocal();
  if (stored && (!queryName || queryName === stored.name)) {
    let status;
    try {
      status = await api("/api/sessions/status?name=" + encodeURIComponent(stored.name));
    } catch (err) {
      showGate({
        mode: "enter",
        name: stored.name,
        ttl: stored.ttlMinutes,
        message: err.message,
        messageKind: "error",
      });
      return;
    }
    if (!status.exists) {
      showExpired(stored);
      return;
    }
    if (!stored.paused) {
      try {
        const state = await api(sessionURL(stored.name, "/state"), { password: stored.password });
        await showRoom(stored.password, state.session, state.items);
        return;
      } catch (err) {
        if (err.status === 410) {
          showExpired(stored);
          return;
        }
        if (err.status !== 401) {
          showGate({
            mode: "join",
            name: stored.name,
            hasPassword: status.has_password,
            ttl: status.ttl_minutes,
            message: err.message,
            messageKind: "error",
          });
          return;
        }
      }
    }
    showGate({
      mode: "join",
      name: stored.name,
      hasPassword: status.has_password,
      ttl: status.ttl_minutes,
    });
    return;
  }
  showGate({ mode: "enter", name: queryName, ttl: 30 });
}

function init() {
  const curl = document.getElementById("api-curl");
  if (curl) curl.textContent = curl.textContent.replaceAll("HOST", location.origin);
  document.addEventListener("paste", onPaste);
  document.addEventListener("dragenter", (event) => {
    if (!hasFiles(event)) return;
    dragDepth += 1;
    if (document.body.dataset.view === "room") document.body.classList.add("dragging");
  });
  document.addEventListener("dragover", (event) => {
    if (!hasFiles(event)) return;
    event.preventDefault();
  });
  document.addEventListener("dragleave", () => {
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) document.body.classList.remove("dragging");
  });
  document.addEventListener("drop", (event) => {
    if (!hasFiles(event)) return;
    event.preventDefault();
    dragDepth = 0;
    document.body.classList.remove("dragging");
    if (document.body.dataset.view !== "room") return;
    uploadFiles(event.dataTransfer.files);
  });
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden && current) {
      refresh({ keepEditing: true }).catch((err) => {
        if (err.status === 401 || err.status === 410) handleRoomError(err);
      });
    }
  });
  boot();
}

init();
