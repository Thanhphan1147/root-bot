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
      const ch = VB_CHARACTERS[p.Character];
      extra = `<div class="row"><span>character</span><span>${ch ? ch.name : p.Character}</span></div>` +
        (ch ? `<div class="vbability"><b>${ch.ability}:</b> ${ch.text}</div>` : "") +
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
    const ruinLabel = c.Ruin ? `<span class="ruin">ruin</span>` : "";
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

// The engine plays every faction you do not. In 1v1 both sides run the fair,
// determinized MCTS; in the full four-player game the two new factions use a
// 1-ply greedy policy (handy for validating the notation).
const SIMS = 300;
const PROFILES_1V1 = { MC: "mcts:material", ED: "mcts:eyrie" };
const PROFILES_4P = { MC: "greedy:material", ED: "greedy:eyrie", WA: "greedy:material", VB: "greedy:material" };

let chosenSide = "MC";
let chosenMode = "1v1";

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

function syncStart() {
  for (const b of document.querySelectorAll("#modepick button")) {
    b.classList.toggle("on", b.dataset.mode === chosenMode);
  }
  for (const b of document.querySelectorAll("#sidepick button")) {
    // Marquise/Eyrie are playable in both modes; WA/VB only in the full game.
    const show = !b.dataset.mode || b.dataset.mode === chosenMode;
    b.hidden = !show;
    b.classList.toggle("on", show && b.dataset.side === chosenSide);
  }
}
for (const b of document.querySelectorAll("#modepick button")) {
  b.onclick = () => {
    chosenMode = b.dataset.mode;
    const visible = [...document.querySelectorAll("#sidepick button")]
      .filter((x) => !x.dataset.mode || x.dataset.mode === chosenMode);
    if (!visible.some((x) => x.dataset.side === chosenSide)) chosenSide = visible[0].dataset.side;
    syncStart();
  };
}
for (const b of document.querySelectorAll("#sidepick button")) {
  b.onclick = () => { chosenSide = b.dataset.side; syncStart(); };
}
syncStart();
document.getElementById("startgame").onclick = () => { hideStart(); newGame(chosenMode, chosenSide); };
document.getElementById("newgame").onclick = () => { if (!thinking) showStart(); };

// Take back the player's last action (and any CPU replies after it).
document.getElementById("undo").onclick = () => {
  if (thinking || !window.RootBot || !game) return;
  let obj;
  try { obj = JSON.parse(RootBot.undo()); } catch (e) { obj = { error: String(e) }; }
  if (obj.error) { toast(obj.error); return; }
  game = obj;
  viewer = game.you || viewer;
  render();
};

// Export the engine's true, unredacted RMN log. A direct download is unreliable
// on mobile browsers, so we show the text in a dialog with Copy (and Share /
// Download where supported).
const exportDlg = document.getElementById("exportdlg");
const exportText = document.getElementById("exporttext");
const exportStatus = document.getElementById("exportstatus");
const exportShare = document.getElementById("exportshare");

document.getElementById("exportrmn").onclick = () => {
  if (!window.RootBot || !game) { toast("No game to export."); return; }
  exportText.value = RootBot.export();
  exportStatus.textContent = "";
  exportDlg.hidden = false;
  exportShare.hidden = !(navigator.canShare && window.File);
  exportText.focus();
  exportText.setSelectionRange(0, 0);
};
document.getElementById("exportclose").onclick = () => { exportDlg.hidden = true; };

document.getElementById("exportcopy").onclick = async () => {
  const text = exportText.value;
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      exportText.focus();
      exportText.select();
      document.execCommand("copy");
    }
    exportStatus.textContent = "Copied " + text.length + " characters.";
  } catch (e) {
    exportText.focus();
    exportText.select();
    exportStatus.textContent = "Copy failed — select the text and copy manually.";
  }
};

document.getElementById("exportdl").onclick = () => {
  try {
    const blob = new Blob([exportText.value], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "root-demo.rmn";
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    exportStatus.textContent = "Download started (if your browser allows it).";
  } catch (e) {
    exportStatus.textContent = "Download not supported here — use Copy.";
  }
};

exportShare.onclick = async () => {
  const text = exportText.value;
  try {
    const file = new File([text], "root-demo.rmn", { type: "text/plain" });
    if (navigator.canShare && navigator.canShare({ files: [file] })) {
      await navigator.share({ files: [file], title: "ROOT RMN" });
    } else {
      await navigator.share({ title: "ROOT RMN", text });
    }
  } catch (e) { /* user cancelled */ }
};

function newGame(mode, human) {
  if (window.RootBot) {
    RootBot.setBots(JSON.stringify(mode === "4p" ? PROFILES_4P : PROFILES_1V1), SIMS);
  }
  runHuman("newGame", [human, Math.floor(Math.random() * 1e9), 0, mode]);
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
  runHuman("apply", [id]);
}

// --- Bot turn playback (animated) ---
const FACTION_NAME = { MC: "Marquise", ED: "Eyrie" };

// Vagabond characters and their special action, shown in the player panel so the
// ability is always visible.
const VB_CHARACTERS = {
  thief:  { name: "Thief",  ability: "Steal",     text: "Exhaust a torch to take a random card from any player in your clearing." },
  tinker: { name: "Tinker", ability: "Day Labor", text: "Exhaust a torch to take a card from the discard pile whose suit matches your clearing (or a bird)." },
  ranger: { name: "Ranger", ability: "Hideout",   text: "Exhaust a torch to repair 3 items, then immediately end Daylight and begin Evening." },
};
const reducedMotion = () => window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// wasmSync runs a blocking WASM call after yielding a frame so the spinner
// paints, and resolves with the raw JSON string.
function wasmSync(fn, args) {
  return new Promise((resolve) => {
    requestAnimationFrame(() => setTimeout(() => {
      let json = "{}";
      try {
        json = RootBot[fn].apply(null, args);
      } catch (e) {
        json = JSON.stringify({ error: String(e) });
      }
      resolve(json);
    }, 0));
  });
}

// runHuman applies the human's move, then plays and animates the bot's turn.
async function runHuman(fn, args) {
  if (thinking || !window.RootBot) return;
  thinking = true;
  const el = document.getElementById("thinking");
  el.hidden = false;
  try {
    let obj;
    try {
      obj = JSON.parse(await wasmSync(fn, args));
    } catch (e) {
      obj = { error: String(e) };
    }
    game = obj;
    viewer = game.you || viewer;
    render();
    if (game.error) { toast(game.error); return; }
    await playBotTurn();
  } finally {
    thinking = false;
    el.hidden = true;
  }
}

async function playBotTurn() {
  for (let i = 0; i < 500; i++) {
    let obj;
    try {
      obj = JSON.parse(await wasmSync("botStep", []));
    } catch (e) {
      break;
    }
    if (!obj || obj.done || obj.error) {
      if (obj && obj.error) toast(obj.error);
      break;
    }
    await animateAction(game, obj.action, obj.state);
    game = obj.state;
    viewer = game.you || viewer;
  }
  hideBanner();
  if (game) render();
}

function showBanner(text, faction) {
  const el = document.getElementById("actionbanner");
  if (!el) return;
  el.getAnimations && el.getAnimations().forEach((a) => a.cancel());
  el.textContent = text;
  el.className = "actionbanner " + (faction || "");
  el.hidden = false;
  if (!reducedMotion()) {
    el.animate(
      [{ opacity: 0, transform: "translate(-50%, 8px)" }, { opacity: 1, transform: "translate(-50%, 0)" }],
      { duration: 160, easing: "ease-out" }
    );
  }
}
function hideBanner() {
  const el = document.getElementById("actionbanner");
  if (!el || el.hidden) return;
  if (reducedMotion()) { el.hidden = true; return; }
  el.getAnimations && el.getAnimations().forEach((a) => a.cancel());
  const a = el.animate(
    [{ opacity: 1, transform: "translate(-50%, 0)" }, { opacity: 0, transform: "translate(-50%, 8px)" }],
    { duration: 140, easing: "ease-in", fill: "forwards" }
  );
  a.onfinish = () => { el.hidden = true; a.cancel(); };
}

const BATTLE_KINDS = ["battle", "decree-battle", "vb-battle-ally", "vb-strike"];
const BUILD_KINDS = ["mc-build", "decree-build", "setup-mc-build"];
// Every action that moves warriors from one clearing to another (the Eyrie's
// Decree Move is "decree-move", not "move").
const MOVE_KINDS = ["move", "decree-move", "wa-move", "organize", "vb-move"];

const GLYPH = {
  sword: '<svg class="gi" viewBox="0 0 20 20" aria-hidden="true"><path d="M10 2v9M6 11h8M10 11v7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>',
  hammer: '<svg class="gi" viewBox="0 0 20 20" aria-hidden="true"><rect x="5" y="2.5" width="10" height="4.5" rx="1" fill="currentColor"/><rect x="9.2" y="6.5" width="1.6" height="11.5" rx="0.8" fill="currentColor"/></svg>',
};

function clearingEl(id) {
  return document.querySelector('.clearing[data-clearing="' + id + '"]');
}

// showClearingCard overlays a notification card on a clearing for the duration
// of an action, sliding it in. hideClearingCard slides it out and removes it.
function showClearingCard(clearing, html, faction) {
  const el = clearingEl(clearing);
  if (!el) return null;
  el.classList.add("flash", faction || "");
  const card = document.createElement("div");
  card.className = "clearingtag " + (faction || "");
  card.innerHTML = html;
  el.appendChild(card);
  if (!reducedMotion()) {
    card.animate(cardKeyframes("in"), { duration: 180, easing: "ease-out" });
  }
  return card;
}

function hideClearingCard(card) {
  return new Promise((resolve) => {
    if (!card) { resolve(); return; }
    if (reducedMotion() || !card.animate) { card.remove(); resolve(); return; }
    const a = card.animate(cardKeyframes("out"), { duration: 150, easing: "ease-in", fill: "forwards" });
    const done = () => { card.remove(); resolve(); };
    a.onfinish = done;
    a.oncancel = done;
  });
}

function cardKeyframes(dir) {
  const tableMode = window.matchMedia && window.matchMedia("(max-width: 720px)").matches;
  if (tableMode || reducedMotion()) {
    return dir === "in" ? [{ opacity: 0 }, { opacity: 1 }] : [{ opacity: 1 }, { opacity: 0 }];
  }
  const center = "translateY(0) scale(1)";
  const off = "translateY(-6px) scale(.97)";
  return dir === "in"
    ? [{ opacity: 0, transform: off }, { opacity: 1, transform: center }]
    : [{ opacity: 1, transform: center }, { opacity: 0, transform: off }];
}

function battleCard(a) {
  const atk = `<span class="tagf ${a.faction}">${a.faction}</span>`;
  const def = a.target ? `<span class="tagf ${a.target}">${a.target}</span>` : "";
  return atk + GLYPH.sword + def;
}
function buildCard(a) {
  const f = `<span class="tagf ${a.faction}">${a.faction}</span>`;
  const name = a.building || (a.faction === "ED" ? "roost" : "");
  const b = name ? `<span class="tagg">${name}</span>` : "";
  return f + GLYPH.hammer + b;
}

function bannerText(a) {
  const who = FACTION_NAME[a.faction] || a.faction;
  if (MOVE_KINDS.includes(a.kind) && a.from && a.to && a.amount) {
    return `${who} moving ${a.amount} warrior${a.amount > 1 ? "s" : ""} ${a.from} → ${a.to}`;
  }
  if (BATTLE_KINDS.includes(a.kind) && a.clearing) {
    return `${who} attacking ${a.target || "?"} in ${a.clearing}`;
  }
  if (BUILD_KINDS.includes(a.kind) && a.clearing) {
    const name = a.building || (a.faction === "ED" ? "roost" : "a building");
    return `${who} building ${name} at ${a.clearing}`;
  }
  return (a.faction ? who + ": " : "") + (a.label || a.id);
}

// withSourceMoved copies the state with the moving warriors already removed from
// the origin clearing (visual step 1: the source count drops).
function withSourceMoved(pre, a) {
  const g = Object.assign({}, pre);
  g.clearings = Object.assign({}, pre.clearings);
  const src = Object.assign({}, pre.clearings[a.from]);
  src.Warriors = Object.assign({}, src.Warriors || {});
  const left = (src.Warriors[a.faction] || 0) - a.amount;
  if (left > 0) src.Warriors[a.faction] = left;
  else delete src.Warriors[a.faction];
  g.clearings[a.from] = src;
  return g;
}

// animateAction plays one engine action: moves slide a dot along the road,
// battles and builds flash the clearing with a notification card, then the
// resulting position is committed.
async function animateAction(pre, a, post) {
  const isMove = MOVE_KINDS.includes(a.kind) && a.from && a.to && a.amount > 0;
  if (isMove) {
    game = withSourceMoved(pre, a); // step 1: origin count drops
    render();
    showBanner(bannerText(a), a.faction);
    await animateDot(a.from, a.to, a); // step 2: dot along the road
    game = post; // step 3: destination count rises
    render();
    await sleep(reducedMotion() ? 0 : 120);
    hideBanner();
    return;
  }
  if (BATTLE_KINDS.includes(a.kind) && a.clearing) {
    showBanner(bannerText(a), a.faction);
    const card = showClearingCard(a.clearing, battleCard(a), a.faction);
    await sleep(reducedMotion() ? 0 : 560);
    await hideClearingCard(card);
    game = post;
    render();
    await sleep(reducedMotion() ? 0 : 100);
    hideBanner();
    return;
  }
  if (BUILD_KINDS.includes(a.kind) && a.clearing) {
    showBanner(bannerText(a), a.faction);
    const card = showClearingCard(a.clearing, buildCard(a), a.faction);
    await sleep(reducedMotion() ? 0 : 360);
    await hideClearingCard(card);
    game = post;
    render();
    await sleep(reducedMotion() ? 0 : 100);
    hideBanner();
    return;
  }
  showBanner(bannerText(a), a.faction);
  await sleep(reducedMotion() ? 0 : 260);
  hideBanner();
  game = post;
  render();
}

function animateDot(from, to, a) {
  return new Promise((resolve) => {
    const board = document.getElementById("board");
    const tableMode = window.matchMedia && window.matchMedia("(max-width: 720px)").matches;
    if (!board || !POS[from] || !POS[to] || tableMode || reducedMotion()) {
      resolve();
      return;
    }
    const dot = document.createElement("div");
    dot.className = "movdot " + a.faction;
    dot.textContent = a.amount;
    dot.style.left = POS[from][0] + "%";
    dot.style.top = POS[from][1] + "%";
    board.appendChild(dot);
    const rect = board.getBoundingClientRect();
    const dx = ((POS[to][0] - POS[from][0]) / 100) * rect.width;
    const dy = ((POS[to][1] - POS[from][1]) / 100) * rect.height;
    const anim = dot.animate(
      [
        { transform: "translate(-50%, -50%)" },
        { transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px))` },
      ],
      { duration: 620, easing: "cubic-bezier(.33,.08,.36,1)", fill: "forwards" }
    );
    const done = () => { dot.remove(); resolve(); };
    anim.onfinish = done;
    anim.oncancel = done;
  });
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