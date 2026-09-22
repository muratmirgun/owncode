const { test } = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const fs = require("node:fs");
function harness(shared = []) {
  const calls = [];
  let id = 100;
  const chrome = {
    runtime: {
      onMessage: { addListener() {} },
      onStartup: { addListener() {} },
    },
    storage: { local: { get: async () => ({ shared }) } },
    tabs: {
      create: async () => ({ id: ++id }),
      remove: async (id) => calls.push(["remove", id]),
    },
    debugger: {
      attach: async (target) => calls.push(["attach", target.tabId]),
      detach: async () => {},
      sendCommand: async (_target, name, args) => {
        calls.push([name, args]);
        return { result: { value: { title: "test" } } };
      },
    },
  };
  const context = vm.createContext({
    chrome,
    console,
    AbortController,
    setTimeout,
    clearTimeout,
    URL,
  });
  vm.runInContext(
    fs.readFileSync(__dirname + "/background.js", "utf8"),
    context,
  );
  return {
    calls,
    run: (request, session = "a") =>
      context.execute({ request, session, expires: Date.now() + 10000 }),
  };
}
test("unshared tabs cannot attach", async () => {
  const h = harness();
  await assert.rejects(h.run({ action: "observe", tab: "12" }), /Share/);
  assert.equal(h.calls.length, 0);
});
test("shared tabs survive close", async () => {
  const h = harness([12]);
  await h.run({ action: "close", tab: "12" });
  assert.equal(h.calls.length, 0);
});
test("workers own distinct tabs", async () => {
  const h = harness();
  await h.run({ action: "open", url: "https://example.com" }, "a");
  await h.run({ action: "open", url: "https://example.com" }, "b");
  assert.deepEqual(
    h.calls.filter((x) => x[0] === "attach").map((x) => x[1]),
    [101, 102],
  );
  await h.run({ action: "close" }, "a");
  assert.deepEqual(h.calls.at(-1), ["remove", 101]);
});
test("navigation rejects privileged URLs", async () => {
  const h = harness();
  await assert.rejects(
    h.run({ action: "open", url: "file:///etc/passwd" }),
    /HTTP/,
  );
  assert.equal(
    h.calls.some((x) => x[0] === "Page.navigate"),
    false,
  );
});

test("owned tabs cannot cross worker boundaries", async () => {
  const h = harness();
  await h.run({ action: "open", url: "https://example.com" }, "a");
  await assert.rejects(h.run({ action: "observe", tab: "101" }, "b"), /Share/);
});
test("new tabs retain the first tab and close only the selected tab", async () => {
  const h = harness();
  await h.run({ action: "open", url: "https://example.com" });
  await h.run({ action: "new_tab", url: "https://example.com/two" });
  assert.deepEqual(
    JSON.parse((await h.run({ action: "tabs" })).text).tabs,
    [101, 102],
  );
  await h.run({ action: "switch_tab", tab: "101" });
  await h.run({ action: "close_tab" });
  assert.deepEqual(JSON.parse((await h.run({ action: "tabs" })).text).tabs, [
    102,
  ]);
});
test("navigation rejects embedded credentials", async () => {
  const h = harness();
  await assert.rejects(
    h.run({ action: "open", url: "https://user:secret@example.com" }),
    /credentials/,
  );
  assert.equal(h.calls.length, 0);
});
