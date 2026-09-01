import React from "react";
import ReactDOM from "react-dom/client";
import "@codesweep-ai/ui/styles/core.css";
import "@codesweep-ai/ui/styles/markdown-content.css";
import "@codesweep-ai/ui/styles/syntax.css";
import "@codesweep-ai/ui/styles/components/app-shell.css";
import "@codesweep-ai/ui/styles/components/button.css";
import "@codesweep-ai/ui/styles/components/card-group.css";
import "@codesweep-ai/ui/styles/components/card.css";
import "@codesweep-ai/ui/styles/components/code-block.css";
import "@codesweep-ai/ui/styles/components/chip.css";
import "@codesweep-ai/ui/styles/components/markdown-viewer.css";
import "@codesweep-ai/ui/styles/components/modal.css";
import "@codesweep-ai/ui/styles/components/search-input.css";
import "@codesweep-ai/ui/styles/components/segmented-control.css";
import "@codesweep-ai/ui/styles/components/status-badge.css";
import "@codesweep-ai/ui/styles/components/theme-toggle.css";
import "./app.css";
import { App } from "./App";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode><App /></React.StrictMode>,
);
