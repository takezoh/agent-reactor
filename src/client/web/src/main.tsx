import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./css/tokens.css";
import "./css/app.css";
import "./css/session-list.css";
import "./css/palette.css";
import "./css/snackbar.css";
import "./css/shell.css";
import "./css/status-icon.css";
import "./css/view.css";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
