// DNS Propagation Checker frontend.
//
// No framework and no build step: the whole app is this file, served straight
// out of the Go binary. State lives in the URL so a configuration can be
// shared as a link.

const $ = (sel) => document.querySelector(sel);

const el = {
  servers: $("#servers"),
  records: $("#records"),
  query: $("#query"),
  share: $("#share"),
  status: $("#status"),
  results: $("#results"),
  summary: $("#footer-summary"),
  theme: $("#theme"),
};

let state = { servers: [], records: [] };
let recordTypes = ["A", "AAAA", "CNAME", "MX", "NS", "SOA", "TXT"];
let limits = { max_servers: 16, max_records: 32 };

// URL state ------------------------------------------------------------

// Base64url of the JSON state, UTF-8 safe in both directions.
function encodeState(value) {
  const bytes = new TextEncoder().encode(JSON.stringify(value));
  let binary = "";
  bytes.forEach((b) => (binary += String.fromCharCode(b)));
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function decodeState(encoded) {
  const padded = encoded.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
  return JSON.parse(new TextDecoder().decode(bytes));
}

function readURLState() {
  const param = new URLSearchParams(location.search).get("settings");
  if (!param) return null;
  try {
    const parsed = decodeState(param);
    if (!Array.isArray(parsed.servers) || !Array.isArray(parsed.records)) {
      return null;
    }
    return {
      servers: parsed.servers.map((s) => ({
        enabled: s.enabled !== false,
        name: String(s.name ?? ""),
        address: String(s.address ?? ""),
      })),
      records: parsed.records.map((r) => ({
        enabled: r.enabled !== false,
        fqdn: String(r.fqdn ?? ""),
        type: String(r.type ?? "A").toUpperCase(),
      })),
    };
  } catch {
    return null;
  }
}

function writeURLState() {
  const url = `${location.pathname}?settings=${encodeState(state)}`;
  history.replaceState(null, "", url);
}

// Rendering ------------------------------------------------------------

function serverRow(server, index) {
  const li = document.createElement("li");
  li.className = `row${server.enabled ? "" : " disabled"}`;
  li.innerHTML = `
    <input type="checkbox" data-field="enabled" ${server.enabled ? "checked" : ""}
           aria-label="Include this server" />
    <input type="text" class="name" data-field="name" placeholder="Label"
           value="${escapeAttr(server.name)}" aria-label="Server label" />
    <input type="text" class="value" data-field="address" placeholder="1.1.1.1"
           value="${escapeAttr(server.address)}" aria-label="Server address"
           spellcheck="false" autocapitalize="off" autocomplete="off" />
    <button type="button" class="remove" data-remove aria-label="Remove server">&times;</button>`;
  li.dataset.index = index;
  return li;
}

function recordRow(record, index) {
  const options = recordTypes
    .map(
      (t) =>
        `<option value="${t}"${t === record.type ? " selected" : ""}>${t}</option>`,
    )
    .join("");

  const li = document.createElement("li");
  li.className = `row${record.enabled ? "" : " disabled"}`;
  li.innerHTML = `
    <input type="checkbox" data-field="enabled" ${record.enabled ? "checked" : ""}
           aria-label="Include this record" />
    <input type="text" class="value" data-field="fqdn" placeholder="example.com"
           value="${escapeAttr(record.fqdn)}" aria-label="Domain name"
           spellcheck="false" autocapitalize="off" autocomplete="off" />
    <select data-field="type" aria-label="Record type">${options}</select>
    <button type="button" class="remove" data-remove aria-label="Remove record">&times;</button>`;
  li.dataset.index = index;
  return li;
}

function render() {
  el.servers.replaceChildren(...state.servers.map(serverRow));
  el.records.replaceChildren(...state.records.map(recordRow));
  writeURLState();
}

// Editing --------------------------------------------------------------

// One delegated listener per list keeps the DOM cheap to re-render.
function wireList(container, key) {
  container.addEventListener("input", (event) => {
    const row = event.target.closest(".row");
    const field = event.target.dataset.field;
    if (!row || !field) return;

    const item = state[key][Number(row.dataset.index)];
    if (event.target.type === "checkbox") {
      item.enabled = event.target.checked;
      row.classList.toggle("disabled", !item.enabled);
    } else {
      item[field] = event.target.value;
    }
    event.target.classList.remove("invalid");
    writeURLState();
  });

  container.addEventListener("change", (event) => {
    if (event.target.tagName === "SELECT") {
      state[key][Number(event.target.closest(".row").dataset.index)].type =
        event.target.value;
      writeURLState();
    }
  });

  container.addEventListener("click", (event) => {
    if (!event.target.closest("[data-remove]")) return;
    state[key].splice(Number(event.target.closest(".row").dataset.index), 1);
    render();
  });
}

document.querySelectorAll("[data-add]").forEach((button) => {
  button.addEventListener("click", () => {
    if (button.dataset.add === "server") {
      if (state.servers.length >= limits.max_servers) {
        return setStatus(`At most ${limits.max_servers} servers.`, "error");
      }
      state.servers.push({ enabled: true, name: "", address: "" });
    } else {
      if (state.records.length >= limits.max_records) {
        return setStatus(`At most ${limits.max_records} records.`, "error");
      }
      state.records.push({ enabled: true, fqdn: "", type: "A" });
    }
    render();
    const list = button.dataset.add === "server" ? el.servers : el.records;
    list.lastElementChild?.querySelector('input[type="text"]')?.focus();
  });
});

// Querying -------------------------------------------------------------

// tone is "", "warn" (servers disagree, which is a finding rather than a
// failure) or "error".
function setStatus(message, tone = "") {
  el.status.textContent = message;
  el.status.classList.toggle("warn", tone === "warn");
  el.status.classList.toggle("error", tone === "error");
}

async function runQuery() {
  const servers = state.servers
    .filter((s) => s.enabled && s.address.trim())
    .map((s) => ({ name: s.name.trim(), address: s.address.trim() }));
  const records = state.records
    .filter((r) => r.enabled && r.fqdn.trim())
    .map((r) => ({ fqdn: r.fqdn.trim(), type: r.type }));

  if (!servers.length) return setStatus("Enable at least one server.", "error");
  if (!records.length) return setStatus("Enable at least one record.", "error");

  el.query.disabled = true;
  setStatus(`Querying ${records.length} × ${servers.length}…`);
  const started = performance.now();

  try {
    const response = await fetch("api/query", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ servers, records }),
    });
    const body = await response.json();

    if (!response.ok) {
      throw new Error(body.error || `request failed (${response.status})`);
    }

    renderResults(body);
    const elapsed = Math.round(performance.now() - started);
    setStatus(
      body.diff
        ? `Servers disagree on some records (${elapsed} ms).`
        : `All servers agree (${elapsed} ms).`,
      body.diff ? "warn" : "",
    );
  } catch (error) {
    el.results.replaceChildren();
    setStatus(error.message, "error");
  } finally {
    el.query.disabled = false;
  }
}

function renderResults(result) {
  const columns = result.records[0]?.answers ?? [];

  const table = document.createElement("table");
  const head = table.createTHead().insertRow();
  head.appendChild(th("Record"));
  head.appendChild(th(""));
  columns.forEach((answer) => {
    const cell = th(answer.server.name || answer.server.address);
    if (answer.server.name && answer.server.name !== answer.server.address) {
      cell.appendChild(meta(answer.server.address));
    }
    head.appendChild(cell);
  });

  const body = table.createTBody();
  for (const record of result.records) {
    const row = body.insertRow();
    row.className = record.diff ? "diff" : "";

    const label = document.createElement("th");
    label.scope = "row";
    label.textContent = record.record.fqdn;
    label.appendChild(span("type", record.record.type));
    row.appendChild(label);

    const badge = row.insertCell();
    badge.appendChild(
      span(
        `badge ${record.diff ? "diff" : "ok"}`,
        record.diff ? "diff" : "match",
      ),
    );

    for (const answer of record.answers) {
      row.appendChild(answerCell(answer));
    }
  }

  el.results.replaceChildren(table);
  el.summary.textContent = `${result.records.length} record(s) · ${columns.length} server(s) · ${result.duration_ms} ms`;
}

function answerCell(answer) {
  const cell = document.createElement("td");

  if (answer.error) {
    cell.appendChild(span("failed", answer.error));
    return cell;
  }
  if (!answer.values.length) {
    cell.appendChild(span("empty", "—"));
  } else {
    const pre = document.createElement("pre");
    pre.textContent = answer.values.join("\n");
    cell.appendChild(pre);
  }
  cell.appendChild(meta(`${answer.rcode} · ${answer.duration_ms} ms`));
  return cell;
}

function th(text) {
  const cell = document.createElement("th");
  cell.scope = "col";
  cell.textContent = text;
  return cell;
}

function span(className, text) {
  const node = document.createElement("span");
  node.className = className;
  node.textContent = text;
  return node;
}

function meta(text) {
  return span("meta", text);
}

function escapeAttr(value) {
  return String(value).replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

// Theme ----------------------------------------------------------------

// No stored preference means "follow the OS"; the first click pins whichever
// theme is not currently showing.
const systemDark = window.matchMedia("(prefers-color-scheme: dark)");

function currentTheme() {
  return (
    document.documentElement.dataset.theme ||
    (systemDark.matches ? "dark" : "light")
  );
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem("theme", theme);
  } catch {}
  labelTheme();
}

function labelTheme() {
  const next = currentTheme() === "dark" ? "light" : "dark";
  el.theme.textContent = currentTheme() === "dark" ? "Light mode" : "Dark mode";
  el.theme.setAttribute("aria-label", `Switch to ${next} mode`);
}

el.theme.addEventListener("click", () =>
  applyTheme(currentTheme() === "dark" ? "light" : "dark"),
);

systemDark.addEventListener("change", () => {
  if (!document.documentElement.dataset.theme) labelTheme();
});

labelTheme();

// Boot -----------------------------------------------------------------

el.query.addEventListener("click", runQuery);

el.share.addEventListener("click", async () => {
  writeURLState();
  try {
    await navigator.clipboard.writeText(location.href);
    setStatus("Link copied.");
  } catch {
    setStatus("Copy the address bar to share this configuration.");
  }
});

document.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter") runQuery();
});

wireList(el.servers, "servers");
wireList(el.records, "records");

(async function boot() {
  try {
    const defaults = await (await fetch("api/defaults")).json();
    recordTypes = defaults.record_types;
    limits = defaults.limits;
    state = readURLState() ?? {
      servers: defaults.servers.map((s) => ({ enabled: true, ...s })),
      records: defaults.records.map((r) => ({ enabled: true, ...r })),
    };
  } catch {
    state = readURLState() ?? {
      servers: [{ enabled: true, name: "Cloudflare", address: "1.1.1.1" }],
      records: [{ enabled: true, fqdn: "example.com", type: "A" }],
    };
    setStatus("Could not load defaults from the server.", "error");
  }
  render();
})();
