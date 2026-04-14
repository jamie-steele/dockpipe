# Resolver example (`example`)

Minimal **resolver-shaped** workflow: one **`skip_container`** step that runs a bundled agnostic script via the **`core.*`** namespace (`scripts/core.assets.scripts.example-resolver-notice.sh`).

Copy this tree when adding a new resolver profile; put real tool integrations under **`packages/<group>/resolvers/<name>/`** (with **`profile`**) in your vendor tree.
