let running = false,
  controller;
const owned = new Map();
const sessionTabs = new Map();
chrome.runtime.onMessage.addListener((m, _sender, reply) => {
  if (m.action === "start") {
    if (running) {
      controller?.abort();
      setTimeout(start, 1200);
    } else {
      start();
    }
    reply({ ok: true });
  }
  if (m.action === "stop") {
    controller?.abort();
    reply({ ok: true });
  }
});
chrome.runtime.onStartup.addListener(start);
async function start() {
  if (running) return;
  running = true;
  controller = new AbortController();
  try {
    while (!controller.signal.aborted) {
      const { pairing } = await chrome.storage.local.get("pairing");
      if (!pairing) break;
      try {
        const response = await fetch(pairing.url + "/next", {
          headers: { Authorization: "Bearer " + pairing.token },
          signal: controller.signal,
        });
        if (response.status === 401 || response.status === 403) break;
        if (response.status === 204) continue;
        if (!response.ok) throw Error("Relay unavailable");
        const job = await response.json();
        if (Date.now() > job.expires) continue;
        let result;
        try {
          result = await execute(job);
        } catch (e) {
          result = { error: e.message };
        }
        await fetch(pairing.url + "/result", {
          method: "POST",
          headers: {
            Authorization: "Bearer " + pairing.token,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ id: job.id, ...result }),
          signal: controller.signal,
        });
      } catch (e) {
        if (controller.signal.aborted) break;
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    }
  } finally {
    running = false;
  }
}
async function execute(job) {
  const r = job.request;
  if (["open", "navigate", "new_tab"].includes(r.action)) {
    const u = new URL(r.url);
    if (!["http:", "https:"].includes(u.protocol) || u.username || u.password)
      throw Error("HTTP or HTTPS URL without credentials required");
  }
  const { shared = [] } = await chrome.storage.local.get("shared");
  let tabID = owned.get(job.session);
  if (r.tab) {
    tabID = Number(r.tab);
    if (!shared.includes(tabID) && !sessionTabs.get(job.session)?.has(tabID))
      throw Error("Share this tab from the extension first");
  }
  if (r.action === "tabs")
    return {
      text: JSON.stringify({
        tabs: [...(sessionTabs.get(job.session) || [])],
        shared,
        active: owned.get(job.session),
      }),
    };
  if (r.action === "switch_tab") {
    if (!r.tab) throw Error("tab is required");
    owned.set(job.session, tabID);
    return { text: "Selected tab " + tabID };
  }
  if (r.action === "close" || r.action === "close_tab") {
    const tabs = sessionTabs.get(job.session);
    if (r.tab && !tabs?.has(tabID)) return { text: "Shared tabs remain open" };
    for (const id of [...(tabs || [])]) {
      if (r.action === "close" || id === tabID) {
        await chrome.tabs.remove(id);
        tabs.delete(id);
      }
    }
    if (!tabs?.size) {
      sessionTabs.delete(job.session);
      owned.delete(job.session);
    } else owned.set(job.session, tabs.values().next().value);
    return { text: "Released owned tabs; shared tabs remain open" };
  }
  if (!tabID || r.action === "new_tab") {
    if (!sessionTabs.has(job.session) && sessionTabs.size >= 8)
      throw Error("Close an unused browser session first");
    if ((sessionTabs.get(job.session)?.size || 0) >= 8)
      throw Error("Close an unused tab first");
    if (!["open", "navigate", "new_tab"].includes(r.action))
      throw Error("Open a URL first or specify a shared tab ID");
    const u = new URL(r.url);
    if (!["http:", "https:"].includes(u.protocol) || u.username || u.password)
      throw Error("HTTP or HTTPS URL without credentials required");
    const tab = await chrome.tabs.create({ url: "about:blank", active: false });
    tabID = tab.id;
    owned.set(job.session, tabID);
    if (!sessionTabs.has(job.session)) sessionTabs.set(job.session, new Set());
    sessionTabs.get(job.session).add(tabID);
  }
  const target = { tabId: tabID };
  await chrome.debugger.attach(target, "1.3");
  try {
    const send = async (method, params = {}, cleanup = false) => {
      if (!cleanup && controller?.signal.aborted) throw Error("Disconnected");
      if (!cleanup && Date.now() > job.expires) throw Error("Action expired");
      let timer;
      try {
        return await Promise.race([
          chrome.debugger.sendCommand(target, method, params),
          new Promise((_, reject) => {
            timer = setTimeout(
              () => reject(Error("Browser command timed out")),
              cleanup ? 1000 : 5000,
            );
          }),
        ]);
      } finally {
        clearTimeout(timer);
      }
    };
    const value = async (expression) => {
      const v = await send("Runtime.evaluate", {
        expression,
        returnByValue: true,
      });
      if (v.exceptionDetails)
        throw Error(v.exceptionDetails.text || "Page operation failed");
      return v.result.value;
    };
    const sel = r.element
      ? '[data-owncode-ref="' + Number(r.element) + '"]'
      : r.selector;
    if (
      [
        "click",
        "hover",
        "double_click",
        "right_click",
        "drag",
        "press",
        "scroll",
      ].includes(r.action)
    )
      await send("Page.bringToFront");
    switch (r.action) {
      case "open":
      case "navigate":
      case "new_tab":
        if (!/^https?:\/\//.test(r.url))
          throw Error("HTTP or HTTPS URL required");
        await send("Page.navigate", { url: r.url });
        return {
          text: "Navigation started. Use observe to inspect the loaded page.",
        };
      case "reload":
        await send("Page.reload");
        return { text: "Reload started" };
      case "back":
      case "forward": {
        const h = await send("Page.getNavigationHistory"),
          entry = h.entries[h.currentIndex + (r.action === "back" ? -1 : 1)];
        if (!entry) throw Error("No history entry");
        await send("Page.navigateToHistoryEntry", { entryId: entry.id });
        return { text: "Navigation started" };
      }
      case "dialog":
        await send("Page.handleJavaScriptDialog", {
          accept: !!r.accept,
          promptText: r.text || "",
        });
        return { text: "Dialog handled" };
      case "wait": {
        if (!sel) throw Error("selector required");
        while (
          !(await value(
            `!!document.querySelector(${JSON.stringify(sel)})?.getClientRects().length`,
          ))
        ) {
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
        break;
      }
      case "upload": {
        if (!sel || !r.files?.length)
          throw Error("selector and files required");
        const { root } = await send("DOM.getDocument");
        const { nodeId } = await send("DOM.querySelector", {
          nodeId: root.nodeId,
          selector: sel,
        });
        if (!nodeId) throw Error("File input missing");
        await send("DOM.setFileInputFiles", { nodeId, files: r.files });
        break;
      }
      case "click":
      case "hover":
      case "double_click":
      case "right_click":
      case "drag": {
        if (!sel) throw Error("selector required");
        const point = (selector, scroll = true) =>
          value(
            `(()=>{const e=document.querySelector(${JSON.stringify(selector)});if(!e)throw Error('Element missing');if(${scroll})e.scrollIntoView({block:'center',inline:'center'});const b=e.getBoundingClientRect();if(!b.width||!b.height||b.x+b.width/2<0||b.y+b.height/2<0||b.x+b.width/2>=innerWidth||b.y+b.height/2>=innerHeight)throw Error('Element must be visible in the viewport');return {x:b.x+b.width/2,y:b.y+b.height/2}})()`,
          );
        let from = await point(sel),
          to = from;
        if (r.action === "drag") {
          if (!r.targetSelector) throw Error("targetSelector required");
          to = await point(r.targetSelector);
          from = await point(sel);
          to = await point(r.targetSelector, false);
        }
        const button = r.action === "right_click" ? "right" : "left";
        await send("Input.dispatchMouseEvent", { type: "mouseMoved", ...from });
        if (r.action === "hover") break;
        let pressed = false;
        try {
          for (
            let count = 1;
            count <= (r.action === "double_click" ? 2 : 1);
            count++
          ) {
            pressed = true;
            await send("Input.dispatchMouseEvent", {
              type: "mousePressed",
              ...from,
              button,
              clickCount: count,
            });
            if (r.action === "drag")
              for (let i = 1; i <= 20; i++)
                await send("Input.dispatchMouseEvent", {
                  type: "mouseMoved",
                  x: from.x + ((to.x - from.x) * i) / 20,
                  y: from.y + ((to.y - from.y) * i) / 20,
                  button,
                  buttons: 1,
                });
            await send("Input.dispatchMouseEvent", {
              type: "mouseReleased",
              ...to,
              button,
              clickCount: count,
            });
            pressed = false;
          }
        } finally {
          if (pressed)
            await send(
              "Input.dispatchMouseEvent",
              { type: "mouseReleased", ...to, button, clickCount: 1 },
              true,
            ).catch(() => {});
        }
        break;
      }
      case "observe":
        break;
      case "type":
      case "select":
        if (!sel) throw Error("element or selector required");
        await value(
          `(()=>{const e=document.querySelector(${JSON.stringify(sel)});if(!e)throw Error('Element missing');if(e.disabled||e.readOnly)throw Error('Element cannot be edited');e.focus();const text=${JSON.stringify(r.text || "")};if(e.isContentEditable){e.textContent=text;}else{const proto=e instanceof HTMLTextAreaElement?HTMLTextAreaElement.prototype:e instanceof HTMLSelectElement?HTMLSelectElement.prototype:HTMLInputElement.prototype;Object.getOwnPropertyDescriptor(proto,'value').set.call(e,text);}e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));return true})()`,
        );
        break;
      case "press": {
        const keys = {
          Enter: 13,
          Tab: 9,
          Escape: 27,
          Backspace: 8,
          ArrowUp: 38,
          ArrowDown: 40,
          ArrowLeft: 37,
          ArrowRight: 39,
          Home: 36,
          End: 35,
          PageUp: 33,
          PageDown: 34,
          Delete: 46,
          Space: 32,
          F1: 112,
          F2: 113,
          F3: 114,
          F4: 115,
          F5: 116,
          F6: 117,
          F7: 118,
          F8: 119,
          F9: 120,
          F10: 121,
          F11: 122,
          F12: 123,
        };
        const mods = { Alt: 1, Control: 2, Meta: 4, Shift: 8 };
        let modifiers = 0;
        for (const m of r.modifiers || []) {
          if (!(m in mods)) throw Error("Unsupported modifier");
          modifiers |= mods[m];
        }
        if (!(r.key in keys) && [...(r.key || "")].length !== 1)
          throw Error("Unsupported key");
        const code = keys[r.key] || r.key.toUpperCase().charCodeAt(0);
        await send("Input.dispatchKeyEvent", {
          type: "keyDown",
          key: r.key,
          windowsVirtualKeyCode: code,
          modifiers,
          ...(!(modifiers & 7)
            ? {
                text:
                  r.key === "Enter"
                    ? "\r"
                    : r.key === "Space"
                      ? " "
                      : [...r.key].length === 1
                        ? r.key
                        : "",
              }
            : {}),
        });
        await send("Input.dispatchKeyEvent", {
          type: "keyUp",
          key: r.key,
          windowsVirtualKeyCode: code,
          modifiers,
        });
        break;
      }
      case "scroll":
        await send("Input.dispatchMouseEvent", {
          type: "mouseWheel",
          x: 0,
          y: 0,
          deltaX: r.x || 0,
          deltaY: r.y || 0,
        });
        break;
      case "screenshot":
        return {
          image: (await send("Page.captureScreenshot", { format: "png" })).data,
          text: "Screenshot",
        };
      default:
        throw Error("Unsupported action");
    }
    const observation = await value(
      `(()=>{document.querySelectorAll("[data-owncode-ref]").forEach(e=>e.removeAttribute("data-owncode-ref"));const es=[...document.querySelectorAll('a,button,input,textarea,select,[role="button"]')].filter(e=>e.getClientRects().length).slice(0,120);return {url:location.href,title:document.title,text:document.body?.innerText.slice(0,10000),elements:es.map((e,i)=>{e.setAttribute('data-owncode-ref',String(i+1));return {element:i+1,tag:e.tagName,label:(e.getAttribute('aria-label')||e.innerText||e.getAttribute('placeholder')||'').slice(0,160)}})}})()`,
    );
    return { text: JSON.stringify(observation) };
  } finally {
    await chrome.debugger.detach(target).catch(() => {});
  }
}
