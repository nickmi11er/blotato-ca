import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./app/App";
import "./shared/styles/tokens.css";
import "./shared/styles/global.css";

import "./app/layout.css";
import "./feature/automation/styles/editor.css";
import "./feature/automation/styles/list.css";
import "./shared/ui/Modal.css";
import "./feature/automation/styles/dialogs.css";
import "./shared/styles/feedback.css";
import "./app/responsive.css";
import "./feature/automation/styles/workflow.css";
import "./feature/email-templates/templates.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
