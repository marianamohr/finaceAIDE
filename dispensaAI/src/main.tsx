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

createRoot(document.getElementById("root")!).render(<App />);
