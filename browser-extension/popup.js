const status = document.querySelector("#status");
document.querySelector("#connect").onclick = async () => {
  try {
    const p = JSON.parse(document.querySelector("#pairing").value);
    const u = new URL(p.url);
    if (
      u.protocol !== "http:" ||
      u.hostname !== "127.0.0.1" ||
      !u.port ||
      u.username ||
      u.password ||
      !/^[a-f0-9]{64}$/.test(p.token)
    )
      throw Error("Invalid pairing data");
    const health = await fetch(u.origin + "/health", {
      headers: { Authorization: "Bearer " + p.token },
      signal: AbortSignal.timeout(3000),
    });
    if (!health.ok)
      throw Error("OwnCode is unavailable or pairing has expired");
    await chrome.storage.local.set({
      pairing: { url: u.origin, token: p.token },
    });
    await chrome.runtime.sendMessage({ action: "start" });
    document.querySelector("#pairing").value = "";
    status.textContent =
      "Connected. Use Share this tab to grant existing-tab access.";
  } catch (e) {
    status.textContent = e.message;
  }
};
document.querySelector("#share").onclick = async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab || !/^https?:/.test(tab.url || "")) {
    status.textContent = "Select a website tab.";
    return;
  }
  const { shared = [] } = await chrome.storage.local.get("shared");
  await chrome.storage.local.set({ shared: [...new Set([...shared, tab.id])] });
  status.textContent = "Shared tab ID: " + tab.id;
};
document.querySelector("#stop").onclick = async () => {
  await chrome.storage.local.remove(["pairing", "shared"]);
  await chrome.runtime.sendMessage({ action: "stop" });
  status.textContent = "Disconnected";
};
