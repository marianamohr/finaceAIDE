import { createRoot } from "react-dom/client";
import App from "./App.tsx";
import "./index.css";

function setAppHeight() {
  const h = window.innerHeight;
  document.documentElement.style.setProperty("--app-height", `${h}px`);
  document.documentElement.style.height = `${h}px`;
  document.body.style.height = `${h}px`;
  const root = document.getElementById("root");
  if (root) root.style.height = `${h}px`;
}
setAppHeight();
window.addEventListener("resize", setAppHeight);

try {
  const raw = localStorage.getItem("fai-a");
  if (raw) {
    const parsed = JSON.parse(raw);
    if (parsed?.state?.token) {
      delete parsed.state.token;
      localStorage.setItem("fai-a", JSON.stringify(parsed));
    }
  }
} catch {}

createRoot(document.getElementById("root")!).render(<App />);
