use std::env;
use tauri::{LogicalSize, Manager, WebviewUrl, WebviewWindowBuilder};

#[tauri::command]
fn read_clipboard_text() -> Result<String, String> {
    let mut clipboard = arboard::Clipboard::new().map_err(|e| e.to_string())?;
    clipboard.get_text().map_err(|e| e.to_string())
}

#[tauri::command]
fn write_clipboard_text(text: String) -> Result<(), String> {
    let mut clipboard = arboard::Clipboard::new().map_err(|e| e.to_string())?;
    clipboard.set_text(text).map_err(|e| e.to_string())
}

const CLIPBOARD_BRIDGE_JS: &str = r#"
(() => {
  const invoke =
    window.__TAURI_INTERNALS__ &&
    typeof window.__TAURI_INTERNALS__.invoke === 'function'
      ? window.__TAURI_INTERNALS__.invoke
      : null;

  if (!invoke || window.__PIPEON_CLIPBOARD_PATCHED__) {
    return;
  }
  window.__PIPEON_CLIPBOARD_PATCHED__ = true;

  const bridge = {
    async readText() {
      return await invoke('read_clipboard_text');
    },
    async writeText(text) {
      await invoke('write_clipboard_text', { text: String(text ?? '') });
    }
  };

  try {
    if (navigator.clipboard) {
      navigator.clipboard.readText = bridge.readText;
      navigator.clipboard.writeText = bridge.writeText;
    } else {
      Object.defineProperty(navigator, 'clipboard', {
        value: bridge,
        configurable: true
      });
    }
  } catch (_) {
    try {
      Object.defineProperty(navigator, 'clipboard', {
        value: bridge,
        configurable: true
      });
    } catch (_) {}
  }

  window.__PIPEON_HOST__ = Object.assign({}, window.__PIPEON_HOST__, {
    clipboard: bridge
  });
})();
"#;

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            read_clipboard_text,
            write_clipboard_text
        ])
        .on_page_load(|window, _payload| {
            let _ = window.eval(CLIPBOARD_BRIDGE_JS);
        })
        .setup(|app| {
            let url = env::var("PIPEON_URL")
                .unwrap_or_else(|_| "http://127.0.0.1:38421/".to_string());
            let title = env::var("PIPEON_WINDOW_TITLE").unwrap_or_else(|_| "Pipeon".to_string());

            let parsed = url::Url::parse(&url)
                .map_err(|e| -> Box<dyn std::error::Error> { Box::new(e) })?;

            let window = WebviewWindowBuilder::new(app, "main", WebviewUrl::External(parsed))
                .title(&title)
                .inner_size(1440.0, 960.0)
                .min_inner_size(1024.0, 720.0)
                .resizable(true)
                .focused(true)
                .build()
                .map_err(|e| -> Box<dyn std::error::Error> { Box::new(e) })?;

            let _ = window.set_size(LogicalSize::new(1440.0, 960.0));
            let _ = window.eval(CLIPBOARD_BRIDGE_JS);
            let _ = window.show();
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("pipeon desktop app failed");
}
