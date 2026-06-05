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
const serverCards = [...document.querySelectorAll("[data-server-card]")];
const editButtons = [...document.querySelectorAll("[data-edit-server]")];
const configSelect = document.querySelector("[data-config-server]");
const configAddress = document.querySelector("[data-config-address]");

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
}

function timeText() {
  return new Date().toLocaleTimeString("zh-CN", { hour12: false });
}

function addMiniLog(level, message) {
  // Home no longer shows a log card; keep this as a quiet hook for future status toasts.
}

function addTerminal(level, message, tone = "") {
  const line = document.createElement("p");
  line.className = tone;
  const time = document.createElement("time");
  const badge = document.createElement("b");
  const text = document.createElement("span");
  time.textContent = `[${timeText()}]`;
  badge.textContent = level;
  text.textContent = message;
  line.append(time, badge, text);
  terminal.append(line);
}

function selectServerByName(name) {
  selectedNode.textContent = name;
  if (configSelect) {
    const options = [...configSelect.options];
    const option = options.find((item) => item.textContent === name);
    if (option) {
      configSelect.value = option.value;
    }
  }
  if (configAddress) {
    configAddress.value = "";
  }
}

function selectCard(card) {
  serverCards.forEach((item) => item.classList.toggle("is-selected", item === card));
  selectServerByName(card.dataset.name, card.dataset.address);
  addMiniLog("INFO", `选择：${card.dataset.name}`);
  addTerminal("INFO", `选择：${card.dataset.name}`);
}

function openServerEditor(card) {
  selectCard(card);
  switchPage("config");
  addMiniLog("INFO", `编辑：${card.dataset.name}`);
}

navButtons.forEach((button) => {
  button.addEventListener("click", () => switchPage(button.dataset.pageTarget));
});

themeToggle.addEventListener("click", () => {
  setTheme(document.body.dataset.theme === "dark" ? "light" : "dark");
});

connectButton.addEventListener("click", () => {
  const running = !connectButton.classList.contains("is-running");
  connectButton.classList.toggle("is-running", running);
  connectButton.setAttribute("aria-pressed", String(running));
  connectLabel.textContent = running ? "已连接" : "点击连接";
  statusChip.classList.toggle("is-running", running);
  statusText.textContent = running ? "已连接" : "未连接";
  addMiniLog(running ? "INFO" : "WARN", running ? "已启动" : "已停止");
  addTerminal(running ? "INFO" : "WARN", running ? "ECH 已连接。" : "已停止。", running ? "" : "warn");
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

document.querySelectorAll("[data-node-option]").forEach((option) => {
  option.addEventListener("click", () => {
    selectServerByName(option.dataset.nodeOption);
    nodeMenu.classList.remove("is-open");
    nodePicker.setAttribute("aria-expanded", "false");
    addMiniLog("INFO", `切换：${option.dataset.nodeOption}`);
  });
});

serverCards.forEach((card) => {
  card.addEventListener("click", () => selectCard(card));
  card.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      selectCard(card);
    }
  });
});

editButtons.forEach((button) => {
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    openServerEditor(button.closest("[data-server-card]"));
  });
});

modeButtons.forEach((button) => {
  button.addEventListener("click", () => {
    modeButtons.forEach((item) => item.classList.toggle("is-active", item === button));
    addTerminal("INFO", `模式：${button.dataset.modeOption}`);
  });
});

document.querySelector("[data-speed-test]").addEventListener("click", () => {
  addMiniLog("INFO", "测速：45ms");
  addTerminal("INFO", "测速：45ms。");
});

document.querySelector("[data-new-server]").addEventListener("click", () => {
  serverCards.forEach((item) => item.classList.remove("is-selected"));
  selectServerByName("新服务器", "");
  if (configAddress) {
    configAddress.value = "";
    configAddress.placeholder = "服务器地址";
  }
  switchPage("config");
  addMiniLog("INFO", "新建草稿");
});

document.querySelector("[data-back-proxies]").addEventListener("click", () => {
  switchPage("proxies");
});

document.querySelector("[data-delete-server]").addEventListener("click", () => {
  if (serverCards[0]) {
    selectCard(serverCards[0]);
  }
  switchPage("proxies");
  addMiniLog("WARN", "已删除");
});

document.querySelector("[data-clear-logs]").addEventListener("click", () => {
  const line = document.createElement("p");
  const time = document.createElement("time");
  const text = document.createElement("span");
  line.className = "muted";
  time.textContent = "[ ]";
  text.textContent = "待命";
  line.append(time, text);
  terminal.replaceChildren(line);
});

const initialPage = window.location.hash.replace("#", "");
if (initialPage && pages.some((page) => page.id === initialPage)) {
  switchPage(initialPage);
}

const queryTheme = new URLSearchParams(window.location.search).get("theme");
setTheme(queryTheme || document.body.dataset.theme);
