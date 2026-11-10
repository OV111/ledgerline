import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";

// index.html has <div id="root">; React draws the whole app inside it.
// The "!" tells TypeScript we are sure the element exists.
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
