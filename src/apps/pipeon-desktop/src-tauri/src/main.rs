use std::env;
use tauri::{LogicalSize, WebviewUrl, WebviewWindowBuilder};

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

const LAYOUT_SEED_JS: &str = r#"
(() => {
  if (window.__PIPEON_LAYOUT_SEED_STARTED__) {
    return;
  }
  window.__PIPEON_LAYOUT_SEED_STARTED__ = true;

  const SEED_VERSION = 'pipeon-layout-v3';
  const SEED_KEY = 'pipeon.layoutSeedVersion';
  const RELOAD_KEY = `${SEED_KEY}.reloaded`;
  const DORKPIPE_CONTAINER = 'workbench.view.extension.dorkpipe-panel';
  const DORKPIPE_VIEW = 'pipeon.chatView';
  const CHAT_CONTAINER = 'workbench.panel.chat';

  const parseJson = (value, fallback) => {
    if (value == null) {
      return fallback;
    }
    if (typeof value === 'string') {
      try {
        return JSON.parse(value);
      } catch (_) {
        return fallback;
      }
    }
    return fallback;
  };

  const openDatabase = (name) =>
    new Promise((resolve, reject) => {
      const request = indexedDB.open(name);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error || new Error(`Could not open IndexedDB database ${name}`));
    });

  const readKey = (db, key) =>
    new Promise((resolve, reject) => {
      const tx = db.transaction('ItemTable', 'readonly');
      const store = tx.objectStore('ItemTable');
      const request = store.get(key);
      request.onsuccess = () => resolve(request.result ?? null);
      request.onerror = () => reject(request.error || new Error(`Could not read ${key}`));
    });

  const writeKey = (db, key, value) =>
    new Promise((resolve, reject) => {
      const tx = db.transaction('ItemTable', 'readwrite');
      const store = tx.objectStore('ItemTable');
      const request = store.put(value, key);
      request.onsuccess = () => resolve();
      request.onerror = () => reject(request.error || new Error(`Could not write ${key}`));
    });

  const listDbNames = async () => {
    if (typeof indexedDB.databases === 'function') {
      try {
        const dbs = await indexedDB.databases();
        return dbs.map((db) => db && db.name).filter(Boolean);
      } catch (_) {
        // Fall back to the known global database.
      }
    }
    return ['vscode-web-state-db-global'];
  };

  const ensurePinnedPanel = (items, id) => {
    const current = Array.isArray(items) ? items.filter(Boolean) : [];
    const filtered = current.filter((item) => item && item.id !== id);
    const maxOrder = filtered.reduce((max, item) => {
      const order = Number.isFinite(item?.order) ? item.order : 100;
      return Math.max(max, order);
    }, 100);
    filtered.push({ id, pinned: true, visible: false, order: maxOrder + 1 });
    return filtered;
  };

  const ensurePlaceholder = (items, id, name) => {
    const current = Array.isArray(items) ? items.filter(Boolean) : [];
    if (current.some((item) => item && item.id === id)) {
      return current;
    }
    current.push({ id, name, isBuiltin: false });
    return current;
  };

  const removeId = (items, id) => {
    const current = Array.isArray(items) ? items.filter(Boolean) : [];
    return current.filter((item) => item && item.id !== id);
  };

  const setWorkspaceVisibility = (items, id, visible) => {
    const current = Array.isArray(items) ? items.filter(Boolean) : [];
    const filtered = current.filter((item) => item && item.id !== id);
    filtered.push({ id, visible });
    return filtered;
  };

  const maybeWrite = async (db, key, nextValue, currentValue) => {
    const before = JSON.stringify(currentValue);
    const after = JSON.stringify(nextValue);
    if (before === after) {
      return false;
    }
    await writeKey(db, key, JSON.stringify(nextValue));
    return true;
  };

  const seedGlobalState = async (db) => {
    let changed = false;

    const customizations = parseJson(await readKey(db, 'views.customizations'), {
      viewContainerLocations: {},
      viewLocations: {},
      viewContainerBadgeEnablementStates: {},
    });
    customizations.viewContainerLocations = customizations.viewContainerLocations || {};
    customizations.viewLocations = customizations.viewLocations || {};
    customizations.viewContainerBadgeEnablementStates =
      customizations.viewContainerBadgeEnablementStates || {};

    if (customizations.viewLocations[DORKPIPE_VIEW]) {
      delete customizations.viewLocations[DORKPIPE_VIEW];
      changed = true;
    }
    if (customizations.viewContainerLocations[DORKPIPE_CONTAINER] !== 2) {
      customizations.viewContainerLocations[DORKPIPE_CONTAINER] = 2;
      changed = true;
    }
    if (changed) {
      await writeKey(db, 'views.customizations', JSON.stringify(customizations));
    }

    const auxPinned = parseJson(await readKey(db, 'workbench.auxiliarybar.pinnedPanels'), []);
    changed =
      (await maybeWrite(
        db,
        'workbench.auxiliarybar.pinnedPanels',
        ensurePinnedPanel(auxPinned, DORKPIPE_CONTAINER),
        auxPinned
      )) || changed;

    const auxPlaceholders = parseJson(await readKey(db, 'workbench.auxiliarybar.placeholderPanels'), []);
    changed =
      (await maybeWrite(
        db,
        'workbench.auxiliarybar.placeholderPanels',
        ensurePlaceholder(auxPlaceholders, DORKPIPE_CONTAINER, 'DorkPipe'),
        auxPlaceholders
      )) || changed;

    const panelPinned = parseJson(await readKey(db, 'workbench.panel.pinnedPanels'), []);
    changed =
      (await maybeWrite(
        db,
        'workbench.panel.pinnedPanels',
        removeId(panelPinned, DORKPIPE_CONTAINER),
        panelPinned
      )) || changed;

    const panelChatHidden = parseJson(await readKey(db, 'workbench.panel.chat.hidden'), []);
    changed =
      (await maybeWrite(
        db,
        'workbench.panel.chat.hidden',
        removeId(panelChatHidden, DORKPIPE_VIEW),
        panelChatHidden
      )) || changed;

    return changed;
  };

  const seedWorkspaceState = async (dbName) => {
    if (!dbName || dbName === 'vscode-web-state-db-global') {
      return false;
    }

    const db = await openDatabase(dbName);
    let changed = false;

    const auxState = parseJson(await readKey(db, 'workbench.auxiliarybar.viewContainersWorkspaceState'), []);
    changed =
      (await maybeWrite(
        db,
        'workbench.auxiliarybar.viewContainersWorkspaceState',
        setWorkspaceVisibility(auxState, DORKPIPE_CONTAINER, true),
        auxState
      )) || changed;

    const panelState = parseJson(await readKey(db, 'workbench.panel.viewContainersWorkspaceState'), []);
    changed =
      (await maybeWrite(
        db,
        'workbench.panel.viewContainersWorkspaceState',
        removeId(panelState, DORKPIPE_CONTAINER),
        panelState
      )) || changed;

    const panelChatState = parseJson(await readKey(db, 'workbench.panel.chat'), {});
    if (panelChatState && typeof panelChatState === 'object' && DORKPIPE_VIEW in panelChatState) {
      delete panelChatState[DORKPIPE_VIEW];
      await writeKey(db, 'workbench.panel.chat', JSON.stringify(panelChatState));
      changed = true;
    }

    return changed;
  };

  const seedLayout = async () => {
    try {
      if (localStorage.getItem(SEED_KEY) === SEED_VERSION) {
        return;
      }

      const dbNames = await listDbNames();
      let changed = false;

      if (dbNames.includes('vscode-web-state-db-global')) {
        const globalDb = await openDatabase('vscode-web-state-db-global');
        changed = (await seedGlobalState(globalDb)) || changed;
      }

      for (const name of dbNames) {
        if (!name || !name.startsWith('vscode-web-state-db-') || name === 'vscode-web-state-db-global') {
          continue;
        }
        changed = (await seedWorkspaceState(name)) || changed;
      }

      localStorage.setItem(SEED_KEY, SEED_VERSION);
      if (changed && !sessionStorage.getItem(RELOAD_KEY)) {
        sessionStorage.setItem(RELOAD_KEY, '1');
        location.reload();
      }
    } catch (error) {
      console.warn('[pipeon] Failed to seed DorkPipe layout', error);
    }
  };

  void seedLayout();
})();
"#;

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            read_clipboard_text,
            write_clipboard_text
        ])
        .on_page_load(|window, _payload| {
            let _ = window.eval(LAYOUT_SEED_JS);
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
            let _ = window.eval(LAYOUT_SEED_JS);
            let _ = window.eval(CLIPBOARD_BRIDGE_JS);
            let _ = window.show();
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("pipeon desktop app failed");
}
