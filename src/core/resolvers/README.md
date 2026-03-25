# Core resolvers (lean)

**`templates/core/resolvers/`** in the shipped tree holds **only** the **`example/`** reference layout plus a minimal **`onepassword/`** profile (1Password CLI hints for **`workflow_type: secretstore`** flows). Both are small and safe to copy.

**Other tool- and product-specific profiles** (Claude, Codex, VS Code, Cursor, Ollama, …) live under **`.staging/resolvers/`** in this repo (authored like **`.staging/workflows/`**). The runner resolves **`--resolver`** and **`scripts/…`** against **staging** first, then lean **`templates/core/resolvers/`**.

See **`docs/architecture-model.md`** and **`docs/templates-core-assets.md`**.
