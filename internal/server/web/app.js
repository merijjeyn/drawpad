// app.js — frontend glue between the Excalidraw React component and the
// local Go HTTP server. Everything is plain ES2017 so there is no build step.
//
// Flow:
//   1. fetch /api/init    → prompt + initial scene
//   2. mount <Excalidraw> with that scene
//   3. user edits, optionally types a comment
//   4. on Send: read live scene, export PNG via exportToBlob, POST /api/submit
//   5. on Cancel or window beforeunload: POST /api/cancel (sendBeacon)

(function () {
  "use strict";

  const E = window.ExcalidrawLib;
  if (!E || !E.Excalidraw) {
    showStatus("Failed to load Excalidraw bundle. Check your network.", true);
    return;
  }
  const { Excalidraw, exportToBlob } = E;
  const h = React.createElement;

  // Singleton handle to the live Excalidraw API so the Send button can pull
  // the latest scene without going through React state.
  let apiHandle = null;

  function App({ initialData, onReady }) {
    return h("div", { style: { width: "100%", height: "100%" } },
      h(Excalidraw, {
        initialData,
        excalidrawAPI: (api) => {
          apiHandle = api;
          onReady && onReady();
        },
        UIOptions: {
          canvasActions: {
            // Keep the toolbar uncluttered for short interactions. The user
            // can still save manually if they want via right-click export.
            loadScene: false,
            saveAsImage: false,
            saveToActiveFile: false,
          },
        },
      })
    );
  }

  function showStatus(text, persistent) {
    const el = document.getElementById("status");
    el.textContent = text;
    el.classList.add("show");
    if (!persistent) {
      setTimeout(() => el.classList.remove("show"), 1800);
    }
  }

  async function fetchInitial() {
    const resp = await fetch("/api/init", { cache: "no-store" });
    if (!resp.ok) throw new Error("init failed: HTTP " + resp.status);
    return resp.json();
  }

  function blobToDataURL(blob) {
    return new Promise((resolve, reject) => {
      const r = new FileReader();
      r.onload = () => resolve(r.result);
      r.onerror = () => reject(r.error);
      r.readAsDataURL(blob);
    });
  }

  async function captureScreenshot() {
    if (!apiHandle) return "";
    const elements = apiHandle.getSceneElements();
    const appState = apiHandle.getAppState();
    const files = apiHandle.getFiles();
    try {
      const blob = await exportToBlob({
        elements,
        appState: { ...appState, exportBackground: true, exportWithDarkMode: false },
        files,
        mimeType: "image/png",
        // Higher = sharper; 2 is the Excalidraw default for "Save image".
        getDimensions: (w, h) => ({ width: w * 2, height: h * 2, scale: 2 }),
      });
      if (!blob) return "";
      return await blobToDataURL(blob);
    } catch (err) {
      console.error("exportToBlob failed:", err);
      return "";
    }
  }

  async function submit() {
    if (!apiHandle) return;
    setBusy(true);
    showStatus("Capturing diagram…", true);
    try {
      const screenshotPng = await captureScreenshot();
      const scene = {
        type: "excalidraw",
        version: 2,
        source: "draw_interface-frontend",
        elements: apiHandle.getSceneElements(),
        appState: pickSerializableAppState(apiHandle.getAppState()),
        files: apiHandle.getFiles() || {},
      };
      const comment = document.getElementById("comment").value;
      const resp = await fetch("/api/submit", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ scene, comment, screenshotPng }),
      });
      if (!resp.ok) {
        const body = await resp.text();
        throw new Error("submit failed: HTTP " + resp.status + " " + body);
      }
      showStatus("Sent. You can close this window.", true);
      submittedCleanly = true;
      // Try to close the window — works in browsers launched in --app mode.
      // If it fails (regular tab), the status text tells the user what to do.
      setTimeout(() => window.close(), 300);
    } catch (err) {
      console.error(err);
      showStatus("Send failed: " + err.message, true);
      setBusy(false);
    }
  }

  let submittedCleanly = false;

  function cancel() {
    setBusy(true);
    showStatus("Cancelling…", true);
    // sendBeacon survives even if the page is unloading; the regular fetch
    // is a back-up for when the user clicked Cancel instead of closing.
    const body = JSON.stringify({ reason: "user cancelled" });
    try {
      navigator.sendBeacon("/api/cancel", new Blob([body], { type: "application/json" }));
    } catch (_) {}
    fetch("/api/cancel", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
    }).catch(() => {}).finally(() => {
      submittedCleanly = true;
      setTimeout(() => window.close(), 200);
    });
  }

  // Excalidraw's appState has non-serialisable bits (e.g. open menus, collab
  // pointers). Strip them so the server sees a clean snapshot.
  function pickSerializableAppState(s) {
    if (!s) return {};
    const out = {};
    const keep = [
      "viewBackgroundColor", "currentItemFontFamily", "currentItemFontSize",
      "currentItemStrokeColor", "currentItemBackgroundColor", "gridSize",
      "theme", "name",
    ];
    for (const k of keep) if (k in s) out[k] = s[k];
    return out;
  }

  function setBusy(busy) {
    document.getElementById("send-btn").disabled = busy;
    document.getElementById("cancel-btn").disabled = busy;
  }

  // Wire up buttons before React mount so they exist even if Excalidraw
  // takes a moment to load.
  document.getElementById("send-btn").addEventListener("click", submit);
  document.getElementById("cancel-btn").addEventListener("click", cancel);
  window.addEventListener("beforeunload", () => {
    if (submittedCleanly) return;
    try {
      navigator.sendBeacon("/api/cancel",
        new Blob([JSON.stringify({ reason: "window closed" })],
          { type: "application/json" }));
    } catch (_) {}
  });

  // Keyboard: Cmd/Ctrl+Enter to send.
  document.addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      submit();
    }
  });

  // ---- boot -------------------------------------------------------------
  fetchInitial().then((payload) => {
    document.getElementById("prompt-text").textContent =
      payload.prompt || "Edit the diagram and click Send when you're done.";

    const initialData = payload.scene
      ? {
          elements: payload.scene.elements || [],
          appState: payload.scene.appState || {},
          files: payload.scene.files || {},
          scrollToContent: true,
        }
      : null;

    ReactDOM.render(
      h(App, { initialData, onReady: () => showStatus("Ready.", false) }),
      document.getElementById("root")
    );
  }).catch((err) => {
    console.error(err);
    showStatus("Failed to load initial scene: " + err.message, true);
  });
})();
