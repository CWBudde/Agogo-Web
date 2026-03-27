import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { ErrorBoundary } from "./components/error-boundary";
import { EngineProvider } from "./wasm/context";
import "./styles.css";

const devLocalHosts = new Set(["localhost", "127.0.0.1", "::1"]);
const devServiceWorkerResetKey = "agogo:dev-service-worker-reset";

async function clearDevServiceWorkers() {
  if (!import.meta.env.DEV || !("serviceWorker" in navigator)) {
    return;
  }

  if (!devLocalHosts.has(window.location.hostname)) {
    return;
  }

  const registrations = await navigator.serviceWorker.getRegistrations();
  if (registrations.length === 0) {
    return;
  }

  const results = await Promise.all(registrations.map((registration) => registration.unregister()));
  if ("caches" in window) {
    const keys = await caches.keys();
    await Promise.all(keys.map((key) => caches.delete(key)));
  }

  if (results.some(Boolean) && !window.sessionStorage.getItem(devServiceWorkerResetKey)) {
    window.sessionStorage.setItem(devServiceWorkerResetKey, "1");
    window.location.reload();
  }
}

void clearDevServiceWorkers().catch((error) => {
  console.warn("Failed to clear service workers for local development.", error);
});

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ErrorBoundary>
      <EngineProvider>
        <App />
      </EngineProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
