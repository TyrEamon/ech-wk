const pages = [...document.querySelectorAll(".page")];
const navButtons = [...document.querySelectorAll("[data-page-target]")];
const connectButton = document.querySelector("[data-connect-button]");
const connectLabel = document.querySelector("[data-connect-label]");
const statusChip = document.querySelector("[data-status-chip]");
const statusText = document.querySelector("[data-status-text]");
const themeToggle = document.querySelector("[data-theme-toggle]");
const themeLabel = document.querySelector("[data-theme-label]");
const nodePicker = document.querySelector("[data-node-picker]");
const nodeMenu = document.querySelector("[data-node-menu]");
const selectedNode = document.querySelector("[data-selected-node]");
const modeButtons = [...document.querySelectorAll("[data-mode-option]")];
const terminal = document.querySelector("[data-terminal]");
const serverList = document.querySelector("[data-server-list]");
const searchInput = document.querySelector("[data-search]");
const configSelect = document.querySelector("[data-config-server]");
const speedButton = document.querySelector("[data-speed-test]");
const speedLabel = document.querySelector("[data-speed-label]");

const fields = {
  name: document.querySelector("[data-config-name]"),
  server: document.querySelector("[data-config-address]"),
  listen: document.querySelector("[data-config-listen]"),
  token: document.querySelector("[data-config-token]"),
  ip: document.querySelector("[data-config-ip]"),
  dns: document.querySelector("[data-config-dns]"),
  ech: document.querySelector("[data-config-ech]"),
  routing: document.querySelector("[data-config-routing]"),
};

let state = {
  servers: [],
  current_server_id: "",
  running: false,
  system_proxy_enabled: false,
  logs: [],
};

let speedTesting = false;

function appApi() {
  return window.go?.main?.App || null;
}

async function invoke(method, ...args) {
  const api = appApi();
  if (!api) {
    return fallback(method, ...args);
  }
  return api[method](...args);
}

async function fallback(method, ...args) {
  if (!state.servers.length) {
    const server = defaultServer();
    state.servers = [server];
    state.current_server_id = server.id;
  }
  if (method === "GetState") return state;
  if (method === "StartProxy") {
    state.running = true;
    pushLog("INFO", "预览模式：已连接。");
  }
  if (method === "StopProxy") {
    state.running = false;
    pushLog("WARN", "预览模式：已停止。", "warn");
  }
  if (method === "SelectServer") {
    state.current_server_id = args[0];
  }
  if (method === "SetRoutingMode") {
    currentServer().routing_mode = args[0];
    pushLog("INFO", `模式：${routingLabel(args[0])}。`);
  }
  if (method === "SaveServer") {
    const server = args[0];
    const index = state.servers.findIndex((item) => item.id === server.id);
    if (index >= 0) state.servers[index] = server;
    else state.servers.push(server);
    state.current_server_id = server.id;
  }
  if (method === "CreateServer") {
    const server = defaultServer(`preview-${Date.now()}`, `新服务器${state.servers.length}`);
    state.servers.push(server);
    state.current_server_id = server.id;
  }
  if (method === "DeleteServer") {
    if (state.servers.length > 1) {
      state.servers = state.servers.filter((item) => item.id !== args[0]);
      state.current_server_id = state.servers[0].id;
    }
  }
  if (method === "TestLatency") {
    currentServer().latency_ms = 45;
    pushLog("INFO", "真实链路：45ms。");
  }
  if (method === "ClearLogs") state.logs = [];
  return state;
}

function defaultServer(id = "default", name = "默认服务器") {
  return {
    id,
    name,
    server: "",
    listen: "127.0.0.1:30000",
    token: "",
    ip: "",
    dns: "dns.alidns.com/dns-query",
    ech: "cloudflare-ech.com",
    routing_mode: "bypass_cn",
    latency_ms: 0,
  };
}

function pushLog(level, message, tone = "") {
  state.logs.push({
    time: new Date().toLocaleTimeString("zh-CN", { hour12: false }),
    level,
    message,
    tone,
  });
}

function currentServer() {
  return state.servers.find((server) => server.id === state.current_server_id) || state.servers[0] || defaultServer();
}

function navKey(pageId) {
  return pageId === "config" ? "proxies" : pageId;
}

function switchPage(pageId) {
  pages.forEach((page) => page.classList.toggle("is-active", page.id === pageId));
  const activeKey = navKey(pageId);
  navButtons.forEach((button) => {
    button.classList.toggle("is-active", button.dataset.pageTarget === activeKey);
  });
}

function setTheme(theme) {
  const normalized = theme === "light" ? "light" : "dark";
  document.body.dataset.theme = normalized;
  themeToggle.setAttribute("aria-pressed", String(normalized === "light"));
  themeLabel.textContent = normalized === "light" ? "日间" : "夜间";
  localStorage.setItem("ech-theme", normalized);
}

function applyState(nextState) {
  if (!nextState) return;
  state = {
    servers: nextState.servers || [],
    current_server_id: nextState.current_server_id || "",
    running: Boolean(nextState.running),
    system_proxy_enabled: Boolean(nextState.system_proxy_enabled),
    logs: nextState.logs || [],
  };
  if (!state.servers.length) {
    const server = defaultServer();
    state.servers = [server];
    state.current_server_id = server.id;
  }
  render();
}

function render() {
  const server = currentServer();
  selectedNode.textContent = server.name || "默认服务器";
  statusChip.classList.toggle("is-running", state.running);
  statusText.textContent = state.running ? "已连接" : "未连接";
  connectButton.classList.toggle("is-running", state.running);
  connectButton.setAttribute("aria-pressed", String(state.running));
  connectLabel.textContent = state.running ? "已连接" : "点击连接";

  modeButtons.forEach((button) => {
    button.classList.toggle("is-active", button.dataset.modeOption === (server.routing_mode || "bypass_cn"));
  });

  renderNodeMenu();
  renderServerList();
  renderConfig();
  renderLogs();
}

function renderNodeMenu() {
  nodeMenu.replaceChildren(...state.servers.map((server) => {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = server.name || "未命名服务器";
    const badge = document.createElement("span");
    badge.textContent = server.id === state.current_server_id ? "当前" : "";
    button.append(badge);
    button.addEventListener("click", async () => {
      await updateFromBackend("SelectServer", server.id);
      nodeMenu.classList.remove("is-open");
      nodePicker.setAttribute("aria-expanded", "false");
    });
    return button;
  }));
}

function renderServerList() {
  const query = (searchInput.value || "").trim().toLowerCase();
  const servers = state.servers.filter((server) => {
    return !query || `${server.name} ${server.server}`.toLowerCase().includes(query);
  });
  const group = document.createDocumentFragment();
  const header = document.createElement("div");
  header.className = "group-header";
  header.innerHTML = `<span>组 1</span><strong>${servers.length}</strong>`;
  group.append(header);
  servers.forEach((server) => group.append(createServerCard(server)));
  serverList.replaceChildren(group);
}

function createServerCard(server) {
  const card = document.createElement("article");
  card.className = `server-card${server.id === state.current_server_id ? " is-selected" : ""}`;
  card.tabIndex = 0;
  card.setAttribute("role", "button");
  card.innerHTML = `
    <span class="server-icon${server.id === state.current_server_id ? "" : " dim"}" aria-hidden="true">
      <svg class="globe-svg"><use href="#icon-globe"></use></svg>
    </span>
    <span class="server-info">
      <strong>${escapeHtml(server.name || "未命名服务器")}</strong>
      <small><b>ech</b><i class="${latencyClass(server)}">${latencyLabel(server)}</i></small>
    </span>
    <span class="server-radio"></span>
    <button class="edit-button" type="button" aria-label="编辑 ${escapeHtml(server.name || "服务器")}">
      <svg class="edit-svg"><use href="#icon-pencil"></use></svg>
    </button>
  `;
  card.addEventListener("click", () => updateFromBackend("SelectServer", server.id));
  card.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      updateFromBackend("SelectServer", server.id);
    }
  });
  card.querySelector(".edit-button").addEventListener("click", async (event) => {
    event.stopPropagation();
    await updateFromBackend("SelectServer", server.id);
    switchPage("config");
  });
  return card;
}

function latencyLabel(server) {
  const latency = Number(server.latency_ms || 0);
  return latency > 0 ? `${latency}ms` : "未测速";
}

function latencyClass(server) {
  const latency = Number(server.latency_ms || 0);
  if (latency <= 0) return "latency";
  if (latency < 120) return "latency is-good";
  if (latency < 280) return "latency is-ok";
  return "latency is-slow";
}

function renderConfig() {
  const server = currentServer();
  configSelect.replaceChildren(...state.servers.map((item) => {
    const option = document.createElement("option");
    option.value = item.id;
    option.textContent = item.name || "未命名服务器";
    option.selected = item.id === server.id;
    return option;
  }));
  fields.name.value = server.name || "";
  fields.server.value = server.server || "";
  fields.listen.value = server.listen || "";
  fields.token.value = server.token || "";
  fields.ip.value = server.ip || "";
  fields.dns.value = server.dns || "";
  fields.ech.value = server.ech || "";
  fields.routing.value = server.routing_mode || "bypass_cn";
}

function renderLogs() {
  if (!state.logs.length) {
    const line = document.createElement("p");
    line.className = "muted";
    line.innerHTML = `<time>[ ]</time><span>待命</span>`;
    terminal.replaceChildren(line);
    return;
  }
  terminal.replaceChildren(...state.logs.slice(-120).map((entry) => createLogLine(entry)));
  terminal.scrollTop = terminal.scrollHeight;
}

function createLogLine(entry) {
  const line = document.createElement("p");
  line.className = entry.tone || "";
  const time = document.createElement("time");
  const badge = document.createElement("b");
  const text = document.createElement("span");
  time.textContent = `[${entry.time || ""}]`;
  badge.textContent = entry.level || "INFO";
  text.textContent = entry.message || "";
  line.append(time, badge, text);
  return line;
}

function collectServer() {
  const current = currentServer();
  return {
    id: current.id,
    name: fields.name.value.trim() || "未命名服务器",
    server: fields.server.value.trim(),
    listen: fields.listen.value.trim() || "127.0.0.1:30000",
    token: fields.token.value,
    ip: fields.ip.value.trim(),
    dns: fields.dns.value.trim() || "dns.alidns.com/dns-query",
    ech: fields.ech.value.trim() || "cloudflare-ech.com",
    routing_mode: fields.routing.value || "bypass_cn",
  };
}

async function updateFromBackend(method, ...args) {
  try {
    const next = await invoke(method, ...args);
    applyState(next);
    return next;
  } catch (error) {
    const message = error?.message || String(error);
    pushLog("ERROR", message, "error");
    renderLogs();
    return null;
  }
}

function setSpeedTesting(testing) {
  speedTesting = testing;
  speedButton.classList.toggle("is-testing", testing);
  speedButton.disabled = testing;
  speedButton.setAttribute("aria-busy", String(testing));
  speedLabel.textContent = testing ? "测试中" : "测速";
}

function routingLabel(mode) {
  return {
    global: "全局",
    bypass_cn: "绕过大陆",
    none: "不改变",
  }[mode] || mode;
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;",
  }[char]));
}

navButtons.forEach((button) => {
  button.addEventListener("click", () => switchPage(button.dataset.pageTarget));
});

themeToggle.addEventListener("click", () => {
  setTheme(document.body.dataset.theme === "dark" ? "light" : "dark");
});

connectButton.addEventListener("click", async () => {
  await updateFromBackend(state.running ? "StopProxy" : "StartProxy");
});

nodePicker.addEventListener("click", () => {
  const open = !nodeMenu.classList.contains("is-open");
  nodeMenu.classList.toggle("is-open", open);
  nodePicker.setAttribute("aria-expanded", String(open));
});

document.addEventListener("click", (event) => {
  if (!event.target.closest(".selected-card")) {
    nodeMenu.classList.remove("is-open");
    nodePicker.setAttribute("aria-expanded", "false");
  }
});

modeButtons.forEach((button) => {
  button.addEventListener("click", () => updateFromBackend("SetRoutingMode", button.dataset.modeOption));
});

speedButton.addEventListener("click", async () => {
  if (speedTesting) return;
  setSpeedTesting(true);
  try {
    await updateFromBackend("TestLatency");
  } finally {
    setSpeedTesting(false);
  }
});
document.querySelector("[data-new-server]").addEventListener("click", async () => {
  await updateFromBackend("CreateServer");
  switchPage("config");
});
document.querySelector("[data-back-proxies]").addEventListener("click", () => switchPage("proxies"));
document.querySelector("[data-save-server]").addEventListener("click", () => updateFromBackend("SaveServer", collectServer()));
document.querySelector("[data-delete-server]").addEventListener("click", async () => {
  await updateFromBackend("DeleteServer", currentServer().id);
  switchPage("proxies");
});
document.querySelector("[data-clear-logs]").addEventListener("click", () => updateFromBackend("ClearLogs"));
searchInput.addEventListener("input", renderServerList);
configSelect.addEventListener("change", () => updateFromBackend("SelectServer", configSelect.value));

function initEvents() {
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn("proxy:log", (entry) => {
      state.logs.push(entry);
      renderLogs();
    });
    window.runtime.EventsOn("proxy:state", applyState);
  }
}

async function init(retries = 20) {
  setTheme(localStorage.getItem("ech-theme") || document.body.dataset.theme);
  initEvents();
  if (!appApi() && retries > 0) {
    await updateFromBackend("GetState");
    const initialPage = window.location.hash.replace("#", "");
    if (initialPage && pages.some((page) => page.id === initialPage)) {
      switchPage(initialPage);
    }
    setTimeout(() => {
      if (appApi()) init(0);
    }, 100);
    return;
  }
  await updateFromBackend("GetState");
  const initialPage = window.location.hash.replace("#", "");
  if (initialPage && pages.some((page) => page.id === initialPage)) {
    switchPage(initialPage);
  }
}

init();
