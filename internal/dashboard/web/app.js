const $ = (id) => document.getElementById(id);
const money = (n) => "$" + Number(n).toFixed(Number(n) < 1 ? 4 : 2);
const esc = (s) =>
  String(s).replace(/[&<>]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));

async function tick() {
  let s;
  try {
    s = await (await fetch("/api/state")).json();
  } catch (_) {
    $("live").textContent = "disconnected";
    $("live").classList.add("off");
    return;
  }
  $("live").textContent = "● live";
  $("live").classList.remove("off");

  $("spent").textContent = money(s.spent_usd);
  $("budget").textContent = money(s.budget_usd);
  $("window").textContent = s.window_label;

  const pct = s.budget_usd > 0 ? Math.min(100, (100 * s.spent_usd) / s.budget_usd) : 0;
  const over = s.killed || (s.budget_usd > 0 && s.spent_usd >= s.budget_usd);
  $("fill").style.width = pct + "%";
  $("fill").className = "fill" + (over ? " over" : pct > 80 ? " warn" : "");
  $("pct").textContent = pct.toFixed(0) + "%";
  $("status").textContent = s.killed
    ? "KILLED — all requests refused"
    : over
    ? "OVER BUDGET — requests refused until the window clears"
    : "";

  $("sessions").innerHTML =
    (s.sessions || [])
      .map((x) => `<tr><td>${esc(x.session)}</td><td class="num">${money(x.usd)}</td></tr>`)
      .join("") || '<tr><td class="dim">no traffic yet</td></tr>';

  $("recent").innerHTML =
    (s.recent || [])
      .map(
        (e) =>
          `<tr><td class="dim">${esc(e.at)}</td><td>${esc(e.model)}</td>` +
          `<td class="num">${money(e.usd)}${e.estimated ? ' <span class="est">est</span>' : ""}</td></tr>`
      )
      .join("") || '<tr><td class="dim">—</td></tr>';
}

$("kill").addEventListener("click", async () => {
  if (!confirm("Refuse all further requests now?")) return;
  await fetch("/kill", { method: "POST" });
  tick();
});

tick();
setInterval(tick, 1000);
