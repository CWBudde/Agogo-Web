#!/usr/bin/env node

import { spawn } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";

const chromePath = process.env.CHROME_BIN || "google-chrome";
const profileUrl = process.env.AGOGO_PROFILE_URL || "http://127.0.0.1:4173/browser-zoom-profile.html";
const remoteDebugPort = Number(process.env.AGOGO_REMOTE_DEBUG_PORT || 9222);
const iterations = Number(process.env.AGOGO_PROFILE_ITERATIONS || 2000);
const userDataDir = process.env.AGOGO_PROFILE_USER_DATA_DIR || "/tmp/agogo-browser-profile";

function nowMs() {
  return Number(process.hrtime.bigint()) / 1e6;
}

async function fetchJSON(url) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to fetch ${url}: ${response.status} ${response.statusText}`);
  }
  return response.json();
}

async function waitForEndpoint(url, attempts = 50) {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      return await fetchJSON(url);
    } catch {
      await delay(200);
    }
  }
  throw new Error(`Timed out waiting for ${url}`);
}

async function waitForTarget(url, attempts = 50) {
  const listUrl = `http://127.0.0.1:${remoteDebugPort}/json/list`;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    const targets = await fetchJSON(listUrl);
    const target = targets.find((entry) => entry.url === url);
    if (target?.webSocketDebuggerUrl) {
      return target;
    }
    await delay(200);
  }
  throw new Error(`Timed out waiting for target ${url}`);
}

class CdpClient {
  constructor(webSocketUrl) {
    this.socket = new WebSocket(webSocketUrl);
    this.nextId = 1;
    this.pending = new Map();
    this.events = [];
  }

  async connect() {
    await new Promise((resolve, reject) => {
      this.socket.addEventListener("open", resolve, { once: true });
      this.socket.addEventListener("error", reject, { once: true });
    });

    this.socket.addEventListener("message", (event) => {
      const message = JSON.parse(event.data);
      if (typeof message.id === "number") {
        const pending = this.pending.get(message.id);
        if (!pending) {
          return;
        }
        this.pending.delete(message.id);
        if (message.error) {
          pending.reject(new Error(message.error.message || "CDP command failed"));
          return;
        }
        pending.resolve(message.result);
        return;
      }
      this.events.push(message);
    });
  }

  async send(method, params = {}) {
    const id = this.nextId++;
    const payload = JSON.stringify({ id, method, params });
    const promise = new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
    this.socket.send(payload);
    return promise;
  }

  async close() {
    this.socket.close();
  }
}

async function waitForReady(client, attempts = 100) {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    const result = await client.send("Runtime.evaluate", {
      expression:
        "({title: document.title, ready: !!window.__agogoProfile?.ready, status: document.getElementById('status')?.textContent ?? ''})",
      returnByValue: true,
    });
    const value = result.result?.value;
    if (value?.ready === true) {
      return value;
    }
    if (value?.title === "error") {
      throw new Error(`Profiling page failed during setup:\n${value.status}`);
    }
    await delay(200);
  }
  throw new Error("Timed out waiting for profiling page setup");
}

async function timeEval(client, expression) {
  const started = nowMs();
  const result = await client.send("Runtime.evaluate", {
    expression,
    awaitPromise: true,
    returnByValue: true,
  });
  const elapsed = nowMs() - started;
  return { elapsedMs: elapsed, value: result.result?.value };
}

async function main() {
  const chrome = spawn(
    chromePath,
    [
      "--headless=new",
      "--disable-gpu",
      `--remote-debugging-port=${remoteDebugPort}`,
      `--user-data-dir=${userDataDir}`,
      "--no-first-run",
      "--no-default-browser-check",
      profileUrl,
    ],
    {
      stdio: ["ignore", "pipe", "pipe"],
    },
  );

  let chromeStderr = "";
  chrome.stderr.on("data", (chunk) => {
    chromeStderr += chunk.toString();
  });

  try {
    await waitForEndpoint(`http://127.0.0.1:${remoteDebugPort}/json/version`);
    const target = await waitForTarget(profileUrl);
    const client = new CdpClient(target.webSocketDebuggerUrl);
    await client.connect();
    await client.send("Runtime.enable");
    await client.send("Page.enable");
    const ready = await waitForReady(client);

    const setup = await client.send("Runtime.evaluate", {
      expression: "window.__agogoProfile.setup()",
      awaitPromise: true,
      returnByValue: true,
    });

    const renderRaw = await timeEval(client, `window.__agogoProfile.renderFrameRawOnly(${iterations})`);
    const parseOnly = await timeEval(client, `window.__agogoProfile.jsonParseOnly(${iterations})`);
    const render = await timeEval(client, `window.__agogoProfile.renderFrameOnly(${iterations})`);
    const renderHot = await timeEval(client, `window.__agogoProfile.renderFrameHotOnly(${iterations})`);
    const copy = await timeEval(client, `window.__agogoProfile.pixelCopyOnly(${iterations})`);
    const blit = await timeEval(client, `window.__agogoProfile.putImageDataOnly(${iterations})`);
    const endToEnd = await timeEval(client, `window.__agogoProfile.endToEnd(${iterations})`);
    const endToEndSkipReused = await timeEval(
      client,
      `window.__agogoProfile.endToEndSkipReused(${iterations})`,
    );

    const summary = {
      scenario: {
        url: profileUrl,
        iterations,
      },
      setup: setup.result?.value ?? ready,
      timings: {
        renderFrameRawOnly: {
          totalMs: Number(renderRaw.elapsedMs.toFixed(3)),
          perOpMs: Number((renderRaw.elapsedMs / iterations).toFixed(6)),
        },
        jsonParseOnly: {
          totalMs: Number(parseOnly.elapsedMs.toFixed(3)),
          perOpMs: Number((parseOnly.elapsedMs / iterations).toFixed(6)),
        },
        renderFrameOnly: {
          totalMs: Number(render.elapsedMs.toFixed(3)),
          perOpMs: Number((render.elapsedMs / iterations).toFixed(6)),
        },
        renderFrameHotOnly: {
          totalMs: Number(renderHot.elapsedMs.toFixed(3)),
          perOpMs: Number((renderHot.elapsedMs / iterations).toFixed(6)),
        },
        pixelCopyOnly: {
          totalMs: Number(copy.elapsedMs.toFixed(3)),
          perOpMs: Number((copy.elapsedMs / iterations).toFixed(6)),
        },
        putImageDataOnly: {
          totalMs: Number(blit.elapsedMs.toFixed(3)),
          perOpMs: Number((blit.elapsedMs / iterations).toFixed(6)),
        },
        endToEnd: {
          totalMs: Number(endToEnd.elapsedMs.toFixed(3)),
          perOpMs: Number((endToEnd.elapsedMs / iterations).toFixed(6)),
        },
        endToEndSkipReused: {
          totalMs: Number(endToEndSkipReused.elapsedMs.toFixed(3)),
          perOpMs: Number((endToEndSkipReused.elapsedMs / iterations).toFixed(6)),
          presentedFrames: endToEndSkipReused.value?.presented ?? null,
        },
      },
    };

    console.log(JSON.stringify(summary, null, 2));
    await client.close();
  } finally {
    chrome.kill("SIGTERM");
    await Promise.race([
      new Promise((resolve) => chrome.once("exit", resolve)),
      delay(2000),
    ]);
    if (chrome.exitCode !== 0 && chrome.exitCode !== null && chromeStderr.trim()) {
      console.error(chromeStderr.trim());
    }
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.stack ?? error.message : String(error));
  process.exitCode = 1;
});
