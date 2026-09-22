// Demo timeline, injected into out/report.html while it is being recorded.
//
// The buildathon requires the video to be followable without audio, so every
// point is made by a burned-in caption, a highlight, or a scroll -- never by
// narration. Everything shown is the real report rendered from real Nansen
// responses; the script only drives the camera. Total runtime ~53s.
(() => {
  const style = document.createElement("style");
  style.textContent = `
    #demo-cap {
      position: fixed; left: 50%; bottom: 36px; transform: translateX(-50%);
      max-width: 1100px; width: max-content; z-index: 99999;
      padding: 16px 26px; border-radius: 12px;
      background: rgba(8, 10, 16, 0.92); color: #f5f7fb;
      font: 700 28px/1.3 -apple-system, "Inter", "Segoe UI", sans-serif;
      box-shadow: 0 10px 40px rgba(0,0,0,0.6); text-align: center;
      transition: opacity 250ms ease;
    }
    #demo-cap em { color: #fbbf24; font-style: normal; }
    #demo-cap b { color: #fb7185; }
    .demo-hi {
      outline: 3px solid #fbbf24 !important; outline-offset: 4px;
      border-radius: 10px; transition: outline-color 200ms ease;
    }
    #demo-panel {
      position: fixed; inset: 0; z-index: 99998; display: flex;
      align-items: center; justify-content: center;
      background: rgba(8, 10, 16, 0.94); color: #e7eaf2;
      font: 500 22px/1.5 -apple-system, "Inter", "Segoe UI", sans-serif;
      opacity: 0; transition: opacity 400ms ease;
    }
    #demo-panel .box { max-width: 1060px; text-align: center; }
    #demo-panel .big { font-size: 46px; font-weight: 800; color: #fff; }
  `;
  document.head.appendChild(style);

  const cap = document.createElement("div");
  cap.id = "demo-cap";
  cap.style.opacity = "0";
  document.body.appendChild(cap);

  const panel = document.createElement("div");
  panel.id = "demo-panel";
  document.body.appendChild(panel);

  const say = (html) => { cap.innerHTML = html; cap.style.opacity = html ? "1" : "0"; };
  const clearHi = () => document.querySelectorAll(".demo-hi").forEach((e) => e.classList.remove("demo-hi"));
  const hi = (el) => { clearHi(); if (el) el.classList.add("demo-hi"); };
  const scrollTo = (el, offset = 150) => {
    if (!el) return;
    const y = el.getBoundingClientRect().top + window.scrollY - offset;
    window.scrollTo({ top: Math.max(0, y), behavior: "smooth" });
  };
  const row = (sym) => [...document.querySelectorAll("details.token-row")]
    .find((d) => (d.querySelector(".symbol")?.textContent || "").trim() === sym);
  const census = () => document.querySelector(".headline")?.parentElement;

  const steps = [
    [0, () => {
      window.scrollTo(0, 0);
      say("Nansen shows what <em>smart money</em> is buying.<br>Every tool built on it says: copy them.");
    }],
    [5000, () => {
      hi(census());
      say("This one asks a different question:<br><em>is the signal real?</em>");
    }],
    [9500, () => {
      say("<em>67 of 172</em> top smart-money wallets<br>were funded by just <em>13 addresses</em>.");
    }],
    [16000, () => {
      const r = row("ZCAT");
      scrollTo(r, 120); hi(r?.querySelector("summary"));
      say("ZCAT: <em>3</em> “independent” smart-money buyers…");
    }],
    [20500, () => {
      const r = row("ZCAT");
      if (r) { r.open = true; }
      setTimeout(() => {
        const g = r && [...r.querySelectorAll("*")].find((e) =>
          e.children.length === 0 && /shared funder realkingof\.sol/.test(e.textContent));
        hi(g?.closest("div") || r);
        scrollTo(r, 110);
      }, 350);
      say("…two of them were funded by the same address,<br><em>realkingof.sol</em>. 3 buyers → <em>2</em> funding sources.");
    }],
    [27500, () => {
      const z = row("ZCAT"); if (z) z.open = false;
      const r = row("CASHCAT");
      scrollTo(r, 220); hi(r?.querySelector("summary"));
      say("CASHCAT: smart traders net <em>buying +$131,787</em>…<br>while <b>+$3.3M</b> flows <b>into exchanges</b>.");
    }],
    [34000, () => {
      say("You may be buying someone's exit.");
    }],
    [37500, () => {
      const r = row("🌱 X7") || row("X7") ||
        [...document.querySelectorAll("details.token-row")].find((d) => /X7/.test(d.querySelector(".symbol")?.textContent || ""));
      scrollTo(r, 260); hi(r?.querySelector("summary"));
      say("X7 passed both checks: <em>CONFIRMED</em>.");
    }],
    [41500, () => {
      say("But it had already run <b>30x</b> past the smart-money entry.<br>A real signal can still be <em>late</em>.");
    }],
    [47500, () => {
      clearHi(); say("");
      panel.style.opacity = "1";
      panel.innerHTML = `<div class="box">
        <div class="big">smart-money-falsifier</div>
        <div style="margin-top:10px">Don't copy the signal. Try to break it first.</div>
        <div style="margin-top:26px; color:#fbbf24; font-weight:700">
          github.com/watarus/smart-money-falsifier</div>
        <div style="margin-top:6px; color:#8891a7">built on the @nansen_ai API</div></div>`;
    }],
  ];

  for (const [at, fn] of steps) setTimeout(fn, at);
  return "timeline started";
})();
