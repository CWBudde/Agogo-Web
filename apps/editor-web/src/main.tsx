import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { ErrorBoundary } from "./components/error-boundary";
import { EngineProvider } from "./wasm/context";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ErrorBoundary>
      <EngineProvider>
        <App />
      </EngineProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
