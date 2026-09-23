const PHASE = { S: "Setup", B: "Birdsong", D: "Daylight", E: "Evening" };

// Autumn map: clearing positions (percent) and the 18 printed paths.
const POS = {
  C1: [13, 16], C2: [87, 16], C3: [87, 84], C4: [13, 84],
  C5: [50, 10], C6: [90, 48], C7: [57, 82], C8: [29, 88],
  C9: [11, 48], C10: [50, 33], C11: [70, 61], C12: [33, 55],
};
// Forest regions (centroids) and their adjacent clearings.
const FORESTS = {
  AutumnN:  [50, 19],
  AutumnNW: [27, 38],
  AutumnW:  [19, 62],
  AutumnSW: [33, 77],
  AutumnS:  [62, 70],
  AutumnE:  [82, 64],
  Witchwood:[66, 43],
};
const EDGES = [
  ["C1","C5"],["C1","C9"],["C1","C10"],["C2","C5"],["C2","C6"],["C2","C10"],
  ["C3","C6"],["C3","C7"],["C3","C11"],["C4","C8"],["C4","C9"],["C4","C12"],
  ["C6","C11"],["C7","C8"],["C7","C12"],["C9","C12"],["C10","C12"],["C11","C12"],
];

function toast(msg) {
  const el = document.getElementById("pendhint");
  if (el) { el.textContent = msg; setTimeout(() => { if (el.textContent === msg) el.textContent = ""; }, 4000); }
}

// Who must act now: the pending player when the engine is waiting on a deferred
// choice (battle hits, discards, field hospitals), otherwise the turn player.
const PENDING_LABELS = {
  "battle-hits": "assigning battle hits",
  "battle-ambush": "ambush",
  "battle-effects": "battle effects",
  "discard-down": "discarding cards",
  "field-hospitals": "field hospitals",
};

function activeFaction(g) {
  if (g.pending && g.pending.Player) return g.pending.Player;
  return g.current;
}

function actionLabel(g) {
  if (g.setupMode) return "setup";
  if (g.pending && PENDING_LABELS[g.pending.Kind]) return PENDING_LABELS[g.pending.Kind];
  if (g.battle) return "battle · " + (g.battle.StepName || "");
  return (PHASE[g.phase] || g.phase || "").toLowerCase();
}

function renderTurnBanner(g) {
  const el = document.getElementById("turnbanner");
  if (!el) return;
  if (g.winner && g.winner.length) {
    el.hidden = false;
    el.className = "turnbanner win";
    el.innerHTML = `<span class="tb-who">Game over</span><span class="tb-label">${g.winner.join(" + ")} won</span>`;
    return;
  }
  const actor = activeFaction(g);
  if (!actor) { el.hidden = true; return; }
  const yours = actor === viewer;
  el.hidden = false;
  el.className = "turnbanner " + actor + (yours ? " your" : "");
  el.innerHTML =
    `<span class="tb-who">${yours ? "Your turn" : actor + "'s turn"}</span>` +
    `<span class="tb-label">${actionLabel(g)}</span>`;
}

function hideTurnBanner() {
  const el = document.getElementById("turnbanner");
  if (el) el.hidden = true;
}


function renderPlayers(g) {
  const el = document.getElementById("players");
  el.innerHTML = "";
  const actor = activeFaction(g);
  for (const f of g.order) {
    const p = g.players[f];
    const div = document.createElement("div");
    div.className = "pcard " + f + (f === actor ? " current" : "");
    let extra = "";
    if (f === "MC") {
      // Wood on the board (spendable) vs the off-board supply reserve.
      const woodBoard = Object.values(g.clearings).reduce((n, c) => n + (c.Wood || 0), 0);
      extra =
        `<div class="row"><span>wood (board)</span><span>${woodBoard}</span></div>` +
        `<div class="row"><span>wood (supply)</span><span>${p.WoodSupply}</span></div>` +
        `<div class="row"><span>buildings left</span><span>${p.Sawmills}/${p.Workshops}/${p.Recruiters}</span></div>` +
        `<div class="row"><span>keep</span><span>${p.KeepClearing}</span></div>`;
    } else if (f === "ED") {
      extra = `<div class="row"><span>leader</span><span>${p.Leader}</span></div>` +
        `<div class="row"><span>roosts</span><span>${countRoosts(g, "ED")}</span></div>`;
      const dec = p.Decree || {};
      for (const col of ["RECRUIT", "MOVE", "BATTLE", "BUILD"]) {
        const cards = (dec[col] || []).map(cardLabel).join(" ");
        if (cards) extra += `<div class="row"><span>${col.slice(0, 3)}</span><span class="cards">${cards}</span></div>`;
      }
    } else if (f === "WA") {
      extra = `<div class="row"><span>officers</span><span>${p.Officers}</span></div>` +
        `<div class="row"><span>supporters</span><span class="cards">${(p.Supporters || []).map(cardLabel).join(" ")}</span></div>`;
    } else if (f === "VB") {
      extra = `<div class="row"><span>character</span><span>${p.Character}</span></div>` +
        `<div class="row"><span>at</span><span>${p.Pawn}</span></div>` +
        `<div class="row"><span>items</span><span class="cards">${itemList(p)}</span></div>`;
      const rel = p.Relationships || {};
      const tags = Object.entries(rel).map(([k, v]) => `<span class="tag ${v === "hostile" ? "hostile" : ""}">${k}:${v}</span>`).join("");
      extra += `<div class="tags">${tags}</div>`;
    }
    div.innerHTML =
      `<div class="phead"><span class="f">${f}${game.you === f ? " · you" : ""}</span><span class="vp">${p.VP} VP</span></div>` +
      `<div class="pbody">${extra}` +
      `<div class="row"><span>crafted</span><span>${(p.Crafted || []).map(cardLabel).join(" ") || "—"}</span></div>` +
      `</div>`;
    div.append(renderHand(p));
    el.append(div);
  }
}

function renderHand(p) {
  const wrap = document.createElement("div");
  wrap.className = "hand";
  const hand = p.Hand || [];
  if (hand.length === 0) {
    wrap.innerHTML = '<div class="hempty">no cards</div>';
    return wrap;
  }
  for (const id of hand) {
    if (id === "??") {
      const h = document.createElement("div");
      h.className = "hidden-card";
      h.textContent = "hidden card";
      wrap.append(h);
      continue;
    }
    const info = (game.cards && game.cards[id]) || { name: id, suit: "B", desc: "" };
    const c = document.createElement("div");
    c.className = "hcard suit-" + (info.suit || "B");
    c.innerHTML =
      `<div class="hname">${info.name}<span class="hid">${id}</span></div>` +
      `<div class="hcost">${info.cost ? "craft: " + info.cost : (info.kind === "ambush" ? "battle" : info.kind)}</div>` +
      `<div class="hdesc">${info.desc}</div>`;
    wrap.append(c);
  }
  return wrap;
}

function countRoosts(g, f) {
  let n = 0;
  for (const c of Object.values(g.clearings)) {
    for (const b of (c.Buildings || [])) if (b.Owner === f && b.Type === "roost") n++;
  }
  return n;
}

function itemList(p) {
  const out = [];
  for (const [id, it] of Object.entries(p.Items || {})) {
    let s = it.Type;
    if (it.Zone === "track") s += "↑";
    if (!it.FaceUp) s += "×";
    if (it.Damaged) s += "✗";
    out.push(s);
  }
  return out.join(" ");
}

function cardLabel(id) {
  if (id === "VIZIER") return "Viz";
  if (/^[FRMB]\d\d$/.test(id)) return id;
  return id;
}

function renderBoard(g) {
  const el = document.getElementById("board");
  el.innerHTML = "";

  // Roads (SVG underlay).
  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("class", "roads");
  svg.setAttribute("viewBox", "0 0 100 100");
  svg.setAttribute("preserveAspectRatio", "none");
  for (const [a, b] of EDGES) {
    if (!POS[a] || !POS[b]) continue;
    for (const cls of ["casing", "road"]) {
      const ln = document.createElementNS(NS, "line");
      ln.setAttribute("x1", POS[a][0]); ln.setAttribute("y1", POS[a][1]);
      ln.setAttribute("x2", POS[b][0]); ln.setAttribute("y2", POS[b][1]);
      ln.setAttribute("class", cls);
      ln.dataset.c1 = a; ln.dataset.c2 = b;
      svg.append(ln);
    }
  }
  el.append(svg);

  // Forest region labels.
  for (const [name, pos] of Object.entries(FORESTS)) {
    const f = document.createElement("div");
    f.className = "forest";
    f.style.left = pos[0] + "%";
    f.style.top = pos[1] + "%";
    f.textContent = name;
    el.append(f);
  }

  const ids = Object.keys(g.clearings).sort((a, b) => parseInt(a.slice(1)) - parseInt(b.slice(1)));
  const hl = new Set((g.legal || []).map(a => a.clearing || a.to || a.from).filter(Boolean));
  const vbPawn = (g.players && g.players.VB) ? g.players.VB.Pawn : "";

  for (const id of ids) {
    const c = g.clearings[id];
    const div = document.createElement("div");
    div.className = "clearing" + (hl.has(id) ? " hl" : "");
    const [x, y] = POS[id] || [50, 50];
    div.style.left = x + "%";
    div.style.top = y + "%";
    div.dataset.clearing = id;
    div.onmouseenter = () => highlightRoads(svg, id, true);
    div.onmouseleave = () => highlightRoads(svg, id, false);

    let chips = "";
    const order = ["MC", "ED", "WA", "VB"];
    for (const f of order) {
      const n = (c.Warriors || {})[f];
      if (n) chips += `<span class="chip ${f}">${f}×${n}</span>`;
    }
    for (const b of (c.Buildings || [])) chips += `<span class="chip ${b.Owner}">${b.Type}</span>`;
    for (const t of (c.Tokens || [])) chips += `<span class="chip ${t.Owner}">${t.Type}</span>`;
    if (c.Sympathy) chips += `<span class="chip WA">sympathy</span>`;
    if (vbPawn === id) chips += `<span class="chip VB">pawn</span>`;
    const wood = c.Wood ? `<span class="wood">wood ${c.Wood}</span>` : "";
    const slots = c.Slots || 0;
    const used = (c.Buildings || []).length;
    const hasRuin = c.Ruin ? 1 : 0;
    const free = Math.max(0, slots - used - hasRuin);
    let pips = "";
    for (let i = 0; i < used; i++) pips += '<span class="slot used"></span>';
    for (let i = 0; i < hasRuin; i++) pips += '<span class="slot ruinslot"></span>';
    for (let i = 0; i < free; i++) pips += '<span class="slot free"></span>';
    const ruinLabel = c.Ruin ? `<span class="ruin">ruin ${(c.RuinItem || "").replace(/^i\./, "")}</span>` : "";
    const slotRow = (slots || hasRuin)
      ? `<div class="slots" title="building slots: ${free} free of ${slots}"><span class="slotpips">${pips}</span>` +
        `<span class="slotnum">${free}/${slots}</span>${ruinLabel}</div>`
      : "";
    div.innerHTML =
      `<div class="cid"><span>${id}</span><span class="suit ${c.Suit}">${c.Suit}</span></div>` +
      `<div class="crowd">${chips}${wood}</div>${slotRow}`;
    el.append(div);
  }
  // Vagabond pawn in a forest.
  if (vbPawn && FORESTS[vbPawn]) {
    const pos = FORESTS[vbPawn];
    const pd = document.createElement("div");
    pd.className = "pawn";
    pd.style.left = pos[0] + "%";
    pd.style.top = (pos[1] + 8) + "%";
    pd.textContent = "VB pawn";
    pd.title = "Vagabond in " + vbPawn;
    el.append(pd);
  }

  document.getElementById("boardfoot").textContent =
    "roads: " + EDGES.map(([a, b]) => a + "–" + b).join("  ") +
    "   ·   forests: " + Object.keys(FORESTS).join(", ") +
    (vbPawn ? "   ·   VB pawn: " + vbPawn : "");
}

// Compact graph view (shown on small viewports where the board becomes a table).
function renderMinimap(g) {
  const el = document.getElementById("minimap");
  if (!el) return;
  el.innerHTML = "";
  const NS = "http://www.w3.org/2000/svg";
  const SX = 1.6, SY = 1.1; // match the 16:11 board so circles stay round
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", "0 0 160 110");
  svg.setAttribute("preserveAspectRatio", "xMidYMid meet");
  svg.setAttribute("class", "mm-svg");

  const hl = new Set((g.legal || []).map(a => a.clearing || a.to || a.from).filter(Boolean));

  for (const [a, b] of EDGES) {
    if (!POS[a] || !POS[b]) continue;
    const ln = document.createElementNS(NS, "line");
    ln.setAttribute("x1", POS[a][0] * SX); ln.setAttribute("y1", POS[a][1] * SY);
    ln.setAttribute("x2", POS[b][0] * SX); ln.setAttribute("y2", POS[b][1] * SY);
    ln.setAttribute("class", "mm-road");
    svg.append(ln);
  }

  for (const [name, pos] of Object.entries(FORESTS)) {
    const t = document.createElementNS(NS, "text");
    t.setAttribute("x", pos[0] * SX);
    t.setAttribute("y", pos[1] * SY);
    t.setAttribute("class", "mm-forest");
    t.textContent = name === "Witchwood" ? "Witchwood" : name.replace("Autumn", "");
    svg.append(t);
  }

  for (const [id, pos] of Object.entries(POS)) {
    const c = g.clearings[id];
    const suit = c ? c.Suit : "B";
    const node = document.createElementNS(NS, "circle");
    node.setAttribute("cx", pos[0] * SX);
    node.setAttribute("cy", pos[1] * SY);
    node.setAttribute("r", 6);
    node.setAttribute("class", "mm-node mm-suit-" + suit + (hl.has(id) ? " mm-hl" : ""));
    svg.append(node);
    const t = document.createElementNS(NS, "text");
    t.setAttribute("x", pos[0] * SX);
    t.setAttribute("y", pos[1] * SY);
    t.setAttribute("class", "mm-label");
    t.textContent = id.slice(1);
    svg.append(t);
  }

  const vb = g.players && g.players.VB;
  if (vb && vb.Pawn) {
    const p = POS[vb.Pawn] || FORESTS[vb.Pawn];
    if (p) {
      const dot = document.createElementNS(NS, "circle");
      dot.setAttribute("cx", p[0] * SX + 4.2);
      dot.setAttribute("cy", p[1] * SY - 4.2);
      dot.setAttribute("r", 3);
      dot.setAttribute("class", "mm-pawn");
      svg.append(dot);
    }
  }
  el.append(svg);
}

function highlightRoads(svg, id, on) {
  for (const ln of svg.querySelectorAll("line")) {
    if (ln.dataset.c1 === id || ln.dataset.c2 === id) {
      ln.classList.toggle("hot", on);
    }
  }
}

function renderActions(g) {
  const el = document.getElementById("actions");
  el.innerHTML = "";
  const head = document.getElementById("actionhead");
  const pend = document.getElementById("pendhint");
  const actor = activeFaction(g);
  pend.textContent = g.pending ? g.pending.Kind + " (" + g.pending.Player + ")" : "";
  if (g.setupMode) {
    head.textContent = "Setup · " + actor + (g.setupStage ? " · " + g.setupStage : "");
  } else if (g.winner && g.winner.length) {
    head.textContent = "Game over";
  } else if (g.pending) {
    head.textContent = "Actions · " + actor + " · " + (PENDING_LABELS[g.pending.Kind] || g.pending.Kind);
  } else if (g.dayStage === "craft") {
    head.textContent = "Actions · " + actor + " · craft first";
  } else if (g.dayStage === "decree") {
    head.textContent = "Actions · " + actor + " · resolve Decree";
  } else {
    head.textContent = "Actions · " + actor;
  }

  if (g.battle) {
    const b = g.battle;
    const banner = document.createElement("div");
    banner.className = "battlebar";
    const dice = (b.D1 !== undefined && b.D1 !== null) ? ` · dice ${b.D1}-${b.D2}` : "";
    banner.innerHTML = `<b>Battle</b> ${b.Attacker}→${b.Defender} at ${b.Clearing}` +
      `<div class="step">step ${b.Step}/5 · ${b.StepName}${dice}</div>` +
      `<div class="hint">attacker hits ${b.AtkHits || 0} · defender hits ${b.DefHits || 0}` +
      (b.Remaining ? ` · ${b.Remaining} to assign` : "") + `</div>`;
    el.append(banner);
  }

  if (g.winner && g.winner.length) {
    const d = document.createElement("div");
    d.className = "winner";
    d.textContent = "Winner: " + g.winner.join(" + ");
    el.append(d);
    return;
  }
  const acts = g.legal || [];
  if (acts.length === 0) {
    const d = document.createElement("div");
    d.className = "hint";
    if (!(g.winner && g.winner.length)) {
      d.textContent = (game.you && actor === game.you)
        ? "No legal actions."
        : "Waiting for " + actor + "…";
    }
    el.append(d);
    return;
  }
  // Group: pending first, then by kind.
  for (const a of acts) {
    const b = document.createElement("button");
    b.className = (a.kind || "").replace(/:/g, "-");
    b.textContent = a.label || a.id;
    b.onclick = () => doAction(a.id);
    el.append(b);
  }
}

function renderLog(g) {
  const el = document.getElementById("log");
  el.innerHTML = "";
  const entries = g.log || [];
  for (const e of entries.slice(-120)) {
    const li = document.createElement("li");
    li.className = (e.kind || "") + (e.kind === "battle" || e.kind === "turmoil" || e.kind === "ambush" ? " battle" : "");
    li.innerHTML = `<span class="seq">${e.seq}</span><span>${e.round}.${e.phase}</span>` +
      `<span class="act ${e.actor}">${e.actor}</span><span>${e.text}</span>`;
    el.append(li);
  }
  el.scrollTop = el.scrollHeight;
}

function renderRMN(g) {
  const el = document.getElementById("rmn");
  if (!el) return;
  el.innerHTML = "";
  const lines = g.rmn || [];
  if (lines.length === 0) {
    el.innerHTML = '<li class="rmnempty">No RMN events yet.</li>';
    return;
  }
  for (const line of lines) {
    const li = document.createElement("li");
    li.textContent = line;
    el.append(li);
  }
  el.scrollTop = el.scrollHeight;
}

// --- Autofetch ---
const autofetchBox = document.getElementById("autofetch");
const intervalInput = document.getElementById("interval");
const fetchDot = document.getElementById("fetchdot");
// --- solo vs engine controller (WASM) ---

let game = null;
let viewer = "";
let thinking = false;

// The engine plays whichever faction you do not; each side has one tuned bot.
const PROFILES = { MC: "greedy:material", ED: "greedy:eyrie" };

let chosenSide = "MC";

function showStart() {
  document.getElementById("start").hidden = false;
}
function hideStart() {
  document.getElementById("start").hidden = true;
}
function engineReady() {
  const b = document.getElementById("startgame");
  b.disabled = false;
  b.textContent = "Start game";
}
function engineError(msg) {
  const b = document.getElementById("startgame");
  b.disabled = true;
  b.textContent = "Engine failed to load";
  toast(msg);
}

for (const b of document.querySelectorAll("#sidepick button")) {
  b.onclick = () => {
    chosenSide = b.dataset.side;
    for (const x of document.querySelectorAll("#sidepick button")) x.classList.toggle("on", x === b);
  };
}
document.getElementById("startgame").onclick = () => { hideStart(); newGame(chosenSide); };
document.getElementById("newgame").onclick = () => { if (!thinking) showStart(); };

// Export the engine's true, unredacted RMN log for replay and debugging.
document.getElementById("exportrmn").onclick = () => {
  if (!window.RootBot || !game) { toast("No game to export."); return; }
  const text = RootBot.export();
  const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "root-demo.rmn";
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
};

function newGame(human) {
  const botSide = human === "MC" ? "ED" : "MC";
  if (window.RootBot) RootBot.setBot(PROFILES[botSide], 0);
  wasmCall("newGame", [human, Math.floor(Math.random() * 1e9), 0]);
}

// --- Players drawer (small viewports), matching the analysis page ---
const playersToggle = document.getElementById("toggleplayers");
const drawerBackdrop = document.getElementById("drawerbackdrop");
function setDrawer(open) {
  document.body.classList.toggle("players-open", open);
  if (playersToggle) playersToggle.setAttribute("aria-expanded", open ? "true" : "false");
  if (drawerBackdrop) drawerBackdrop.hidden = !open;
}
if (playersToggle) playersToggle.onclick = () => setDrawer(!document.body.classList.contains("players-open"));
if (drawerBackdrop) drawerBackdrop.onclick = () => setDrawer(false);
window.addEventListener("keydown", (e) => { if (e.key === "Escape") setDrawer(false); });

function doAction(id) {
  if (thinking) return;
  wasmCall("apply", [id]);
}

// wasmCall yields a frame so the spinner paints before the blocking WASM call.
function wasmCall(fn, args) {
  if (!window.RootBot) return;
  thinking = true;
  const el = document.getElementById("thinking");
  el.hidden = false;
  requestAnimationFrame(() => setTimeout(() => {
    let json = "{}";
    try {
      json = RootBot[fn].apply(null, args);
    } catch (e) {
      json = JSON.stringify({ error: String(e) });
    }
    try {
      game = JSON.parse(json);
    } catch (e) {
      game = { error: String(e) };
    }
    viewer = game.you || viewer;
    thinking = false;
    el.hidden = true;
    render();
    if (game && game.error) toast(game.error);
  }, 0));
}

function render() {
  if (!game) return;
  const g = game;
  if (g.error && !g.current) {
    document.getElementById("actions").innerHTML = `<div class="winner">${g.error}</div>`;
    return;
  }
  renderTurnBanner(g);
  renderPlayers(g);
  renderMinimap(g);
  renderBoard(g);
  renderActions(g);
  renderLog(g);
  renderRMN(g);
}

// loadEngineBytes prefers a pre-gzipped bundle (GitHub Pages serves the raw
// wasm uncompressed, ~3.9 MB vs ~1 MB gzipped). Falls back to the raw file when
// the browser cannot inflate it.
async function loadEngineBytes() {
  if (typeof DecompressionStream === "function") {
    try {
      const r = await fetch("bot.wasm.gz");
      if (r.ok) {
        const stream = r.body.pipeThrough(new DecompressionStream("gzip"));
        return await new Response(stream).arrayBuffer();
      }
    } catch (e) {
      /* fall through to the raw file */
    }
  }
  return await (await fetch("bot.wasm")).arrayBuffer();
}

async function boot() {
  showStart(); // show the page immediately; Start stays disabled until ready
  try {
    const bytes = await loadEngineBytes();
    const go = new Go();
    const mod = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(mod.instance);
  } catch (e) {
    engineError("Failed to load the engine: " + e);
    return;
  }
  for (let i = 0; i < 600 && !window.RootBot; i++) {
    await new Promise(r => setTimeout(r, 20));
  }
  if (!window.RootBot) {
    engineError("The engine did not start.");
    return;
  }
  engineReady();
}

boot();