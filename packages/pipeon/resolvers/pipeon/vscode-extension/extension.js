"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const vscode = require("vscode");
const fs = require("fs/promises");
const path = require("path");
const http = require("http");
const https = require("https");
const cp = require("child_process");
const os = require("os");
const DEFAULT_OLLAMA_HOST = "http://127.0.0.1:11434";
const DEFAULT_MODEL = "llama3.2";
const CHAT_VIEW_ID = "pipeon.chatView";
const WELCOME_PANEL_ID = "pipeon.welcome";
const PANEL_BOTTOM_MIGRATION_KEY = "pipeon.panelBottomMigrated.v1";
const CHAT_STATE_KEY = "pipeon.chatState.v2";
const MAX_SAVED_SESSIONS = 20;
const MAX_HISTORY_MESSAGES = 14;
const MAX_CONTEXT_CHARS = 18000;
const MAX_SELECTION_CHARS = 2400;
const MAX_MESSAGE_CHARS = 32000;
const MODEL_PROFILES = ["fast", "balanced", "deep", "max"];
const CHAT_WEBVIEW_SHELL_VERSION = "reasoning-templates-v1";
const REASONING_TEMPLATE_SCHEMA = 1;
const BUILTIN_TEMPLATE_ID = "dockpipe.default";
const MAX_REASONING_TEMPLATES = 16;
const MAX_TEMPLATE_NODES = 48;
const MAX_LOOP_ITERATIONS = 8;
function deepClone(value) {
    try {
        return JSON.parse(JSON.stringify(value));
    }
    catch {
        return value;
    }
}
function createDefaultModelEntries() {
    return [
        {
            id: "ollama.default",
            label: "Ollama Default",
            provider: "ollama",
            model: DEFAULT_MODEL,
            builtIn: true,
            locked: true,
            installState: "available",
            capabilities: ["chat", "edit", "reasoning"],
            contextWindow: 8192,
            notes: "Safe local default used by the built-in DockPipe template.",
        },
        {
            id: "ollama.qwen2.5-coder",
            label: "Qwen 2.5 Coder",
            provider: "ollama",
            model: "qwen2.5-coder:14b",
            builtIn: true,
            locked: true,
            installState: "optional",
            capabilities: ["chat", "edit", "code"],
            contextWindow: 16384,
            notes: "Good fit for code-focused model nodes in custom reasoning templates.",
        },
        {
            id: "ollama.deepseek-r1",
            label: "DeepSeek R1",
            provider: "ollama",
            model: "deepseek-r1:14b",
            builtIn: true,
            locked: true,
            installState: "optional",
            capabilities: ["reasoning", "analysis"],
            contextWindow: 16384,
            notes: "Reasoning-heavy option for slower, more deliberate template nodes.",
        },
    ];
}
function firstModelEntryId(modelStore) {
    const entries = Array.isArray(modelStore?.entries) ? modelStore.entries : [];
    return entries[0]?.id || "ollama.default";
}
function defaultGlobalModifiers() {
    return {
        safetyRules: "Prefer deterministic DockPipe paths before model handoff. Fall back safely when validation fails.",
        outputRequirements: "Return inspectable artifacts and preserve current DockPipe artifact validation guarantees.",
        confidenceThreshold: 0.72,
        executionConstraints: "Keep loops bounded and avoid unvalidated side effects.",
        routingPreference: "local-first",
    };
}
function defaultTemplateGuidance() {
    return {
        orchestration: "Route obvious work locally, use models when interpretation is needed, and keep validation first-class.",
        model: "When a model node is used, return structured, parseable output that DockPipe can validate or normalize.",
    };
}
function createDefaultReasoningTemplate(modelStore) {
    const defaultModelId = firstModelEntryId(modelStore);
    return {
        id: BUILTIN_TEMPLATE_ID,
        schema: REASONING_TEMPLATE_SCHEMA,
        name: "DockPipe Default",
        description: "Locked baseline template that mirrors the current DockPipe local-first orchestration flow.",
        builtIn: true,
        locked: true,
        createdAt: nowIso(),
        updatedAt: nowIso(),
        globalModifiers: defaultGlobalModifiers(),
        guidance: defaultTemplateGuidance(),
        nodes: [
            {
                id: "dockpipe-intake",
                type: "dockpipe",
                label: "DockPipe Intake",
                notes: "Performs routing, deterministic primitives, MCP retrieval, and artifact validation.",
                decision: "Prefer local deterministic paths when confidence is high and scope is explicit.",
                guidance: {
                    orchestration: "Build a bounded execution plan and keep artifact generation inspectable.",
                    model: "",
                },
                config: {
                    phase: "route-and-prepare",
                    localFirst: true,
                    artifactMode: "strict",
                },
            },
            {
                id: "model-primary",
                type: "model",
                label: "Primary Model",
                notes: "Handles reasoning or patch generation when DockPipe primitives alone are insufficient.",
                decision: "Only run after DockPipe has assembled the necessary workspace context and routing metadata.",
                guidance: {
                    orchestration: "",
                    model: "Return structured, reliable artifacts suitable for DockPipe validation.",
                },
                config: {
                    modelId: defaultModelId,
                    task: "reason-and-generate",
                    output: "artifact-or-answer",
                },
            },
            {
                id: "loop-repair",
                type: "loop",
                label: "Repair Loop",
                notes: "Bounded repair loop for invalid artifacts or failed validations.",
                decision: "Stop early when the artifact validates cleanly.",
                guidance: {
                    orchestration: "Keep this loop tight and bounded.",
                    model: "",
                },
                config: {
                    maxIterations: 2,
                    stopCondition: "artifact-valid",
                },
                children: [
                    {
                        id: "dockpipe-validate",
                        type: "dockpipe",
                        label: "DockPipe Validate",
                        notes: "Re-check the artifact, normalize it, and decide if repair is still needed.",
                        decision: "Only continue the loop when the artifact is still invalid.",
                        guidance: {
                            orchestration: "Capture diagnostics and keep the failure path inspectable.",
                            model: "",
                        },
                        config: {
                            phase: "validate",
                            localFirst: true,
                            artifactMode: "strict",
                        },
                    },
                    {
                        id: "model-repair",
                        type: "model",
                        label: "Model Repair",
                        notes: "Attempts a bounded repair when validation still fails.",
                        decision: "Repair only the artifact issues surfaced by DockPipe diagnostics.",
                        guidance: {
                            orchestration: "",
                            model: "Return corrected structured output only.",
                        },
                        config: {
                            modelId: defaultModelId,
                            task: "repair",
                            output: "artifact",
                        },
                    },
                ],
            },
        ],
    };
}
function createBlankReasoningTemplate(modelStore, name = "Custom Template") {
    const defaultModelId = firstModelEntryId(modelStore);
    return {
        id: makeId("template"),
        schema: REASONING_TEMPLATE_SCHEMA,
        name,
        description: "Custom reasoning template.",
        builtIn: false,
        locked: false,
        createdAt: nowIso(),
        updatedAt: nowIso(),
        globalModifiers: defaultGlobalModifiers(),
        guidance: defaultTemplateGuidance(),
        nodes: [
            {
                id: makeId("node"),
                type: "dockpipe",
                label: "DockPipe Step",
                notes: "Starting point for a custom template.",
                decision: "",
                guidance: { orchestration: "", model: "" },
                config: {
                    phase: "route-and-prepare",
                    localFirst: true,
                    artifactMode: "strict",
                },
            },
            {
                id: makeId("node"),
                type: "model",
                label: "Model Step",
                notes: "Primary model step.",
                decision: "",
                guidance: { orchestration: "", model: "" },
                config: {
                    modelId: defaultModelId,
                    task: "reason",
                    output: "artifact-or-answer",
                },
            },
        ],
    };
}
function summarizeTemplate(template) {
    if (!template || typeof template !== "object") {
        return null;
    }
    return {
        id: String(template.id || ""),
        name: String(template.name || "Template"),
        builtIn: !!template.builtIn,
        locked: !!template.locked,
        nodeCount: countTemplateNodes(template.nodes || []),
    };
}
async function ensureDir(dir) {
    await fs.mkdir(dir, { recursive: true });
}
function shellQuote(value) {
    return `'${String(value).replace(/'/g, `'\\''`)}'`;
}
function nowIso() {
    return new Date().toISOString();
}
function makeId(prefix) {
    return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}
function clampText(text, maxChars) {
    if (!text)
        return "";
    const value = String(text);
    if (value.length <= maxChars) {
        return value;
    }
    return `${value.slice(0, maxChars)}\n\n[truncated]`;
}
function summarizeDiffPreview(text) {
    const raw = String(text || "").replace(/\r\n/g, "\n");
    if (!raw.trim())
        return "";
    const lines = raw.split("\n");
    const sections = [];
    let current = null;
    for (const line of lines) {
        if (line.startsWith("diff --git ")) {
            if (current)
                sections.push(current);
            current = { file: "", adds: 0, removes: 0, body: [] };
            continue;
        }
        if (!current)
            continue;
        if (line.startsWith("+++ b/")) {
            current.file = line.slice(6).trim();
            continue;
        }
        if (line.startsWith("--- ") ||
            line.startsWith("index ") ||
            line.startsWith("new file mode ") ||
            line.startsWith("@@")) {
            continue;
        }
        if (line.startsWith("+") && !line.startsWith("+++")) {
            current.adds += 1;
            current.body.push(line);
            continue;
        }
        if (line.startsWith("-") && !line.startsWith("---")) {
            current.removes += 1;
            current.body.push(line);
            continue;
        }
    }
    if (current)
        sections.push(current);
    const compact = sections.slice(0, 3).map((section) => {
        const header = `# ${section.file || "file"}  +${section.adds} -${section.removes}`;
        const body = section.body.slice(0, 20).join("\n");
        return [header, body].filter(Boolean).join("\n");
    });
    return clampText(compact.join("\n\n"), 8000);
}
function summarizeTargetFiles(files) {
    const items = Array.isArray(files) ? files.filter(Boolean).map((item) => String(item)) : [];
    if (!items.length)
        return "";
    if (items.length === 1)
        return items[0];
    if (items.length === 2)
        return `${items[0]} and ${items[1]}`;
    return `${items[0]}, ${items[1]}, +${items.length - 2} more`;
}
function pushRecentUnique(items, next, max = 4) {
    const normalized = String(next || "").trim();
    const list = Array.isArray(items) ? items.map((item) => String(item || "")) : [];
    if (!normalized) {
        return list.slice(-max);
    }
    if (list[list.length - 1] === normalized) {
        return list.slice(-max);
    }
    const deduped = [...list.filter((item) => item !== normalized), normalized];
    return deduped.slice(-max);
}
function buildPreparedEditMessage(readyToApply, pendingAction) {
    const count = Array.isArray(readyToApply?.target_files) ? readyToApply.target_files.length : Array.isArray(pendingAction?.targetFiles) ? pendingAction.targetFiles.length : 0;
    const fileSummary = summarizeTargetFiles(readyToApply?.target_files || pendingAction?.targetFiles || []);
    const parts = [];
    if (count > 0) {
        parts.push(`Prepared an edit touching ${count} file${count === 1 ? "" : "s"}.`);
    }
    else {
        parts.push("Prepared an edit for review.");
    }
    if (fileSummary) {
        parts.push(`Files: \`${fileSummary}\`.`);
    }
    const structuredEditCount = Number(pendingAction?.structuredEditCount || readyToApply?.structured_edit_count || 0);
    if (structuredEditCount > 0) {
        const structuredEditTypes = Array.isArray(pendingAction?.structuredEditTypes) && pendingAction.structuredEditTypes.length
            ? pendingAction.structuredEditTypes
            : Array.isArray(readyToApply?.structured_edit_types)
                ? readyToApply.structured_edit_types
                : [];
        parts.push(`Structured plan: ${structuredEditCount} op${structuredEditCount === 1 ? "" : "s"}${structuredEditTypes.length ? ` (${structuredEditTypes.join(", ")})` : ""}.`);
    }
    if (pendingAction?.helperScriptRuntime) {
        parts.push(`Path: bounded sidecar (${pendingAction.helperScriptRuntime}).`);
    }
    else {
        parts.push("Path: primitive or model-assisted edit.");
    }
    parts.push("Review the diff below.");
    return parts.join(" ");
}
function relativeToRoot(root, target) {
    if (!root || !target)
        return null;
    const rel = path.relative(root, target);
    if (!rel || rel.startsWith("..")) {
        return target;
    }
    return rel;
}
function summarizeSessionTitle(text) {
    const oneLine = String(text || "")
        .replace(/\s+/g, " ")
        .replace(/^\/\S+\s*/g, "")
        .trim();
    if (!oneLine) {
        return "New chat";
    }
    if (oneLine.length <= 44) {
        return oneLine;
    }
    return `${oneLine.slice(0, 41)}...`;
}
function createSession(seedText = "") {
    const createdAt = nowIso();
    return {
        id: makeId("chat"),
        title: summarizeSessionTitle(seedText),
        createdAt,
        updatedAt: createdAt,
        messages: [],
    };
}
function createInitialChatState() {
    const session = createSession();
    const modelStore = normalizeModelStore(null);
    const reasoningTemplates = normalizeReasoningTemplates([], modelStore);
    return {
        activeSessionId: session.id,
        sessions: [session],
        composerMode: "ask",
        autoApplyEdits: false,
        modelProfile: "balanced",
        activeTemplateId: BUILTIN_TEMPLATE_ID,
        reasoningTemplates,
        modelStore,
    };
}
function normalizeModelProfile(value) {
    const next = String(value || "").toLowerCase();
    return MODEL_PROFILES.includes(next) ? next : "balanced";
}
function normalizeModelEntry(entry) {
    const id = String(entry?.id || makeId("model")).trim() || makeId("model");
    const label = String(entry?.label || id).trim() || id;
    const provider = String(entry?.provider || "ollama").trim() || "ollama";
    const model = String(entry?.model || DEFAULT_MODEL).trim() || DEFAULT_MODEL;
    const capabilities = Array.isArray(entry?.capabilities)
        ? [...new Set(entry.capabilities.map((item) => String(item || "").trim()).filter(Boolean))].slice(0, 8)
        : [];
    const installState = ["available", "optional", "installed", "custom"].includes(String(entry?.installState || "").toLowerCase())
        ? String(entry.installState).toLowerCase()
        : (entry?.builtIn ? "available" : "custom");
    return {
        id,
        label,
        provider,
        model,
        builtIn: !!entry?.builtIn,
        locked: !!entry?.locked || !!entry?.builtIn,
        installState,
        capabilities,
        contextWindow: Number.isFinite(Number(entry?.contextWindow)) ? Math.max(1024, Number(entry.contextWindow)) : 8192,
        notes: clampText(entry?.notes || "", 600),
    };
}
function normalizeModelStore(raw) {
    const source = Array.isArray(raw?.entries) && raw.entries.length ? raw.entries : createDefaultModelEntries();
    const entries = [];
    const seen = new Set();
    for (const item of source) {
        const normalized = normalizeModelEntry(item);
        if (seen.has(normalized.id)) {
            continue;
        }
        seen.add(normalized.id);
        entries.push(normalized);
    }
    if (!entries.length) {
        entries.push(...createDefaultModelEntries().map(normalizeModelEntry));
    }
    return {
        entries: entries.slice(0, 32),
        updatedAt: String(raw?.updatedAt || nowIso()),
    };
}
function countTemplateNodes(nodes) {
    if (!Array.isArray(nodes))
        return 0;
    return nodes.reduce((sum, node) => sum + 1 + countTemplateNodes(node?.children), 0);
}
function coerceGuidance(value) {
    return {
        orchestration: clampText(value?.orchestration || "", 1200),
        model: clampText(value?.model || "", 1200),
    };
}
function coerceGlobalModifiers(value) {
    const confidence = Number(value?.confidenceThreshold);
    return {
        safetyRules: clampText(value?.safetyRules || defaultGlobalModifiers().safetyRules, 1200),
        outputRequirements: clampText(value?.outputRequirements || defaultGlobalModifiers().outputRequirements, 1200),
        confidenceThreshold: Number.isFinite(confidence) ? Math.min(1, Math.max(0, confidence)) : defaultGlobalModifiers().confidenceThreshold,
        executionConstraints: clampText(value?.executionConstraints || defaultGlobalModifiers().executionConstraints, 1200),
        routingPreference: clampText(value?.routingPreference || defaultGlobalModifiers().routingPreference, 160),
    };
}
function makeDefaultNodeForType(type, modelStore) {
    const modelId = firstModelEntryId(modelStore);
    if (type === "model") {
        return {
            id: makeId("node"),
            type: "model",
            label: "Model Step",
            notes: "",
            decision: "",
            guidance: { orchestration: "", model: "" },
            config: { modelId, task: "reason", output: "artifact-or-answer" },
        };
    }
    if (type === "loop") {
        return {
            id: makeId("node"),
            type: "loop",
            label: "Loop Control",
            notes: "",
            decision: "",
            guidance: { orchestration: "", model: "" },
            config: { maxIterations: 2, stopCondition: "artifact-valid" },
            children: [],
        };
    }
    return {
        id: makeId("node"),
        type: "dockpipe",
        label: "DockPipe Step",
        notes: "",
        decision: "",
        guidance: { orchestration: "", model: "" },
        config: { phase: "route-and-prepare", localFirst: true, artifactMode: "strict" },
    };
}
function normalizeTemplateNode(node, modelStore) {
    const rawType = String(node?.type || "dockpipe").toLowerCase();
    const type = ["dockpipe", "model", "loop"].includes(rawType) ? rawType : "dockpipe";
    const fallback = makeDefaultNodeForType(type, modelStore);
    const normalized = {
        ...fallback,
        id: String(node?.id || fallback.id),
        label: clampText(node?.label || fallback.label, 80),
        notes: clampText(node?.notes || "", 400),
        decision: clampText(node?.decision || "", 400),
        guidance: coerceGuidance(node?.guidance),
    };
    if (type === "dockpipe") {
        normalized.config = {
            phase: clampText(node?.config?.phase || fallback.config.phase, 80),
            localFirst: node?.config?.localFirst !== false,
            artifactMode: clampText(node?.config?.artifactMode || fallback.config.artifactMode, 80),
        };
        return normalized;
    }
    if (type === "model") {
        const available = new Set(normalizeModelStore(modelStore).entries.map((item) => item.id));
        const requestedModelId = String(node?.config?.modelId || fallback.config.modelId);
        normalized.config = {
            modelId: available.has(requestedModelId) ? requestedModelId : firstModelEntryId(modelStore),
            task: clampText(node?.config?.task || fallback.config.task, 80),
            output: clampText(node?.config?.output || fallback.config.output, 80),
        };
        return normalized;
    }
    normalized.config = {
        maxIterations: Math.min(MAX_LOOP_ITERATIONS, Math.max(1, Number(node?.config?.maxIterations || fallback.config.maxIterations) || fallback.config.maxIterations)),
        stopCondition: clampText(node?.config?.stopCondition || fallback.config.stopCondition, 80),
    };
    const rawChildren = Array.isArray(node?.children) ? node.children : [];
    normalized.children = rawChildren.slice(0, MAX_TEMPLATE_NODES).map((child) => normalizeTemplateNode(child, modelStore));
    return normalized;
}
function normalizeReasoningTemplate(template, modelStore, fallbackTemplate) {
    const fallback = fallbackTemplate || createBlankReasoningTemplate(modelStore);
    const id = String(template?.id || fallback.id || makeId("template"));
    const builtIn = id === BUILTIN_TEMPLATE_ID || !!template?.builtIn || !!fallback.builtIn;
    const locked = builtIn || !!template?.locked || !!fallback.locked;
    const nodes = Array.isArray(template?.nodes) && template.nodes.length ? template.nodes : fallback.nodes;
    const normalizedNodes = nodes.slice(0, MAX_TEMPLATE_NODES).map((node) => normalizeTemplateNode(node, modelStore));
    return {
        id,
        schema: REASONING_TEMPLATE_SCHEMA,
        name: clampText(template?.name || fallback.name || "Template", 80),
        description: clampText(template?.description || fallback.description || "", 400),
        builtIn,
        locked,
        createdAt: String(template?.createdAt || fallback.createdAt || nowIso()),
        updatedAt: String(template?.updatedAt || nowIso()),
        globalModifiers: coerceGlobalModifiers(template?.globalModifiers || fallback.globalModifiers),
        guidance: coerceGuidance(template?.guidance || fallback.guidance),
        nodes: normalizedNodes.length ? normalizedNodes : fallback.nodes.map((node) => normalizeTemplateNode(node, modelStore)),
    };
}
function normalizeReasoningTemplates(rawTemplates, modelStore) {
    const defaults = [createDefaultReasoningTemplate(modelStore)];
    const source = Array.isArray(rawTemplates) ? rawTemplates : [];
    const templates = [];
    const seen = new Set();
    for (const item of [...defaults, ...source]) {
        const normalized = normalizeReasoningTemplate(item, modelStore, item?.id === BUILTIN_TEMPLATE_ID ? defaults[0] : undefined);
        if (seen.has(normalized.id)) {
            continue;
        }
        seen.add(normalized.id);
        if (normalized.id === BUILTIN_TEMPLATE_ID) {
            templates.unshift(normalized);
        }
        else {
            templates.push(normalized);
        }
    }
    return templates.slice(0, MAX_REASONING_TEMPLATES);
}
function templateById(templates, templateId) {
    const items = Array.isArray(templates) ? templates : [];
    return items.find((template) => template.id === templateId) || items.find((template) => template.id === BUILTIN_TEMPLATE_ID) || items[0] || null;
}
function resolveNumCtxForProfile(profile) {
    const memGiB = os.totalmem() / (1024 ** 3);
    switch (normalizeModelProfile(profile)) {
        case "fast":
            return memGiB >= 24 ? 4096 : 3072;
        case "deep":
            return memGiB >= 48 ? 24576 : memGiB >= 24 ? 16384 : 8192;
        case "max":
            return memGiB >= 64 ? 32768 : memGiB >= 32 ? 24576 : memGiB >= 24 ? 16384 : 8192;
        default:
            return memGiB >= 32 ? 16384 : memGiB >= 16 ? 8192 : 4096;
    }
}
function modelProfileLabel(profile) {
    switch (normalizeModelProfile(profile)) {
        case "fast":
            return "Fast";
        case "deep":
            return "Deep";
        case "max":
            return "Max";
        default:
            return "Balanced";
    }
}
function sanitizePendingAction(value) {
    if (!value || typeof value !== "object") {
        return null;
    }
    const kind = value.kind === "command" ? "command" : value.kind === "edit" ? "edit" : "";
    if (!kind) {
        return null;
    }
    return {
        kind,
        title: String(value.title || ""),
        artifactDir: value.artifactDir ? String(value.artifactDir) : "",
        patchPath: value.patchPath ? String(value.patchPath) : "",
        tracePath: value.tracePath ? String(value.tracePath) : "",
        artifactVersion: value.artifactVersion ? String(value.artifactVersion) : "",
        structuredEditCount: Number.isFinite(Number(value.structuredEditCount)) ? Math.max(0, Number(value.structuredEditCount)) : 0,
        structuredEditTypes: Array.isArray(value.structuredEditTypes) ? value.structuredEditTypes.map((item) => String(item)).slice(0, 8) : [],
        diffPreview: value.diffPreview ? summarizeDiffPreview(String(value.diffPreview)) : "",
        helperScriptPath: value.helperScriptPath ? String(value.helperScriptPath) : "",
        helperScriptPurpose: value.helperScriptPurpose ? String(value.helperScriptPurpose) : "",
        helperScriptRuntime: value.helperScriptRuntime ? String(value.helperScriptRuntime) : "",
        helperScriptPreview: value.helperScriptPreview ? clampText(String(value.helperScriptPreview), 12000) : "",
        targetFiles: Array.isArray(value.targetFiles) ? value.targetFiles.map((item) => String(item)).slice(0, 8) : [],
        requestText: value.requestText ? String(value.requestText) : "",
        mode: ["ask", "agent", "plan"].includes(String(value.mode || "").toLowerCase()) ? String(value.mode).toLowerCase() : "ask",
    };
}
function sanitizeRunRecord(value) {
    if (!value || typeof value !== "object") {
        return null;
    }
    return {
        artifactDir: value.artifactDir ? String(value.artifactDir) : "",
        patchPath: value.patchPath ? String(value.patchPath) : "",
        tracePath: value.tracePath ? String(value.tracePath) : "",
        artifactVersion: value.artifactVersion ? String(value.artifactVersion) : "",
        summary: value.summary ? clampText(String(value.summary), 1200) : "",
        state: value.state ? String(value.state) : "",
        validationStatus: value.validationStatus ? String(value.validationStatus) : "",
        applyMode: value.applyMode ? String(value.applyMode) : "",
        targetFiles: Array.isArray(value.targetFiles) ? value.targetFiles.map((item) => String(item)).slice(0, 16) : [],
        reasoningKind: value.reasoningKind ? String(value.reasoningKind) : "",
        policy: value.policy && typeof value.policy === "object"
            ? {
                bestOfN: Number.isFinite(Number(value.policy.bestOfN ?? value.policy.best_of_n)) ? Number(value.policy.bestOfN ?? value.policy.best_of_n) : 0,
                maxBranches: Number.isFinite(Number(value.policy.maxBranches ?? value.policy.max_branches)) ? Number(value.policy.maxBranches ?? value.policy.max_branches) : 0,
                abstainThreshold: Number.isFinite(Number(value.policy.abstainThreshold ?? value.policy.abstain_threshold)) ? Number(value.policy.abstainThreshold ?? value.policy.abstain_threshold) : 0,
                repairMemory: !!(value.policy.repairMemory ?? value.policy.repair_memory),
                highAmbiguity: !!(value.policy.highAmbiguity ?? value.policy.high_ambiguity),
                ambiguityReason: value.policy.ambiguityReason || value.policy.ambiguity_reason ? clampText(String(value.policy.ambiguityReason || value.policy.ambiguity_reason), 240) : "",
                branchingActive: !!(value.policy.branchingActive ?? value.policy.branching_active),
            }
            : null,
        structuredEditCount: Number.isFinite(Number(value.structuredEditCount)) ? Math.max(0, Number(value.structuredEditCount)) : 0,
        structuredEditTypes: Array.isArray(value.structuredEditTypes) ? value.structuredEditTypes.map((item) => String(item)).slice(0, 8) : [],
        structuredEdits: Array.isArray(value.structuredEdits)
            ? value.structuredEdits.map((item) => ({
                id: item?.id ? String(item.id) : "",
                op: item?.op ? String(item.op) : "",
                language: item?.language ? String(item.language) : "",
                targetFile: item?.targetFile ? String(item.targetFile) : "",
                description: item?.description ? clampText(String(item.description), 400) : "",
                target: item?.target && typeof item.target === "object"
                    ? {
                        kind: item.target.kind ? String(item.target.kind) : "",
                        symbolName: item.target.symbolName ? String(item.target.symbolName) : "",
                        symbolKind: item.target.symbolKind ? String(item.target.symbolKind) : "",
                    }
                    : null,
                range: item?.range && typeof item.range === "object"
                    ? {
                        startLine: Number.isFinite(Number(item.range.startLine)) ? Number(item.range.startLine) : 0,
                        oldLineCount: Number.isFinite(Number(item.range.oldLineCount)) ? Number(item.range.oldLineCount) : 0,
                        newLineCount: Number.isFinite(Number(item.range.newLineCount)) ? Number(item.range.newLineCount) : 0,
                    }
                    : null,
                preconditions: Array.isArray(item?.preconditions) ? item.preconditions.map((entry) => clampText(String(entry), 180)).slice(0, 4) : [],
                postconditions: Array.isArray(item?.postconditions) ? item.postconditions.map((entry) => clampText(String(entry), 180)).slice(0, 4) : [],
                fallbackNotes: Array.isArray(item?.fallbackNotes) ? item.fallbackNotes.map((entry) => clampText(String(entry), 180)).slice(0, 4) : [],
            })).slice(0, 32)
            : [],
        traceEvents: Array.isArray(value.traceEvents)
            ? value.traceEvents.map((item) => ({
                requestId: item?.requestId ? String(item.requestId) : "",
                parentRequestId: item?.parentRequestId ? String(item.parentRequestId) : "",
                phase: item?.phase ? String(item.phase) : "",
                eventType: item?.eventType ? String(item.eventType) : "",
                label: item?.label ? clampText(String(item.label), 240) : "",
                status: item?.status ? String(item.status) : "",
                progress: Number.isFinite(Number(item?.progress)) ? Number(item.progress) : 0,
                metadata: item?.metadata && typeof item.metadata === "object" ? item.metadata : null,
                timestamp: item?.timestamp ? String(item.timestamp) : "",
            })).slice(-80)
            : [],
        retrievalCandidates: Array.isArray(value.retrievalCandidates)
            ? value.retrievalCandidates.map((item) => ({
                path: item?.path ? String(item.path) : "",
                source: item?.source ? String(item.source) : "",
                selected: !!item?.selected,
                reason: item?.reason ? clampText(String(item.reason), 180) : "",
            })).slice(0, 32)
            : [],
        evidenceNodes: Array.isArray(value.evidenceNodes)
            ? value.evidenceNodes.map((item) => ({
                id: item?.id ? String(item.id) : "",
                kind: item?.kind ? String(item.kind) : "",
                file: item?.file ? String(item.file) : "",
                symbol: item?.symbol ? String(item.symbol) : "",
                summary: item?.summary ? clampText(String(item.summary), 240) : "",
                startLine: Number.isFinite(Number(item?.startLine)) ? Number(item.startLine) : 0,
                endLine: Number.isFinite(Number(item?.endLine)) ? Number(item.endLine) : 0,
            })).slice(0, 48)
            : [],
        evidenceEdges: Array.isArray(value.evidenceEdges)
            ? value.evidenceEdges.map((item) => ({
                from: item?.from ? String(item.from) : "",
                to: item?.to ? String(item.to) : "",
                kind: item?.kind ? String(item.kind) : "",
            })).slice(0, 64)
            : [],
        validationFindings: Array.isArray(value.validationFindings)
            ? value.validationFindings.map((item) => ({
                severity: item?.severity ? String(item.severity) : "",
                code: item?.code ? String(item.code) : "",
                message: item?.message ? clampText(String(item.message), 240) : "",
                nodeIds: Array.isArray(item?.nodeIds) ? item.nodeIds.map((entry) => String(entry)).slice(0, 6) : [],
            })).slice(0, 24)
            : [],
        citations: Array.isArray(value.citations)
            ? value.citations.map((item) => ({
                nodeId: item?.nodeId ? String(item.nodeId) : "",
                file: item?.file ? String(item.file) : "",
                symbol: item?.symbol ? String(item.symbol) : "",
            })).slice(0, 24)
            : [],
        attempts: Array.isArray(value.attempts)
            ? value.attempts.map((item) => ({
                id: item?.id ? String(item.id) : "",
                label: item?.label ? String(item.label) : "",
                kind: item?.kind ? String(item.kind) : "",
                status: item?.status ? String(item.status) : "",
                score: Number.isFinite(Number(item?.score)) ? Number(item.score) : 0,
                summary: item?.summary ? clampText(String(item.summary), 320) : "",
                validationStatus: item?.validationStatus || item?.validation_status ? String(item.validationStatus || item.validation_status) : "",
                failureSummary: item?.failureSummary || item?.failure_summary ? clampText(String(item.failureSummary || item.failure_summary), 240) : "",
                selected: !!item?.selected,
                pruned: !!item?.pruned,
                prunedReason: item?.prunedReason || item?.pruned_reason ? clampText(String(item.prunedReason || item.pruned_reason), 180) : "",
                escalate: !!item?.escalate,
                metadata: item?.metadata && typeof item.metadata === "object" ? item.metadata : null,
            })).slice(0, 16)
            : [],
        decision: value.decision && typeof value.decision === "object"
            ? {
                selectedAttemptId: value.decision.selectedAttemptId || value.decision.selected_attempt_id ? String(value.decision.selectedAttemptId || value.decision.selected_attempt_id) : "",
                abstained: !!value.decision.abstained,
                escalated: !!value.decision.escalated,
                escalationReason: value.decision.escalationReason || value.decision.escalation_reason ? clampText(String(value.decision.escalationReason || value.decision.escalation_reason), 180) : "",
                branchesConsidered: Number.isFinite(Number(value.decision.branchesConsidered ?? value.decision.branches_considered)) ? Number(value.decision.branchesConsidered ?? value.decision.branches_considered) : 0,
                branchesPruned: Number.isFinite(Number(value.decision.branchesPruned ?? value.decision.branches_pruned)) ? Number(value.decision.branchesPruned ?? value.decision.branches_pruned) : 0,
            }
            : null,
        repairMemory: Array.isArray(value.repairMemory) ? value.repairMemory.map((item) => clampText(String(item), 220)).slice(0, 12) : [],
        applyLog: value.applyLog ? clampText(String(value.applyLog), 4000) : "",
        validationLog: value.validationLog ? clampText(String(value.validationLog), 4000) : "",
    };
}
function sanitizeMessage(message) {
    return {
        id: String(message?.id || makeId("msg")),
        role: message?.role === "user" ? "user" : "assistant",
        text: clampText(message?.text || "", MAX_MESSAGE_CHARS),
        format: message?.format === "plain" ? "plain" : "markdown",
        createdAt: String(message?.createdAt || nowIso()),
        pendingAction: sanitizePendingAction(message?.pendingAction),
        run: sanitizeRunRecord(message?.run),
        diffPreview: message?.diffPreview ? summarizeDiffPreview(String(message.diffPreview)) : "",
        liveStatus: message?.liveStatus ? String(message.liveStatus) : "",
        liveTrace: Array.isArray(message?.liveTrace) ? message.liveTrace.map((item) => String(item)).slice(-5) : [],
    };
}
function summarizeRequestActivity(label, mode) {
    const normalizedMode = ["ask", "agent", "plan"].includes(String(mode || "").toLowerCase()) ? String(mode).toLowerCase() : "ask";
    const lower = String(label || "").toLowerCase();
    if (!lower) {
        return normalizedMode === "agent" ? "Working through the request" : "Thinking";
    }
    if (lower.includes("received")) {
        return "Reading your request";
    }
    if (lower.includes("breaking the request into primitives")) {
        return "Breaking the job into primitives";
    }
    if (lower.includes("inspecting workspace context")) {
        return "Scanning the workspace";
    }
    if (lower.includes("package scaffold primitive")) {
        return "Scaffolding the package locally";
    }
    if (lower.includes("collected candidate files")) {
        return "Picking likely files";
    }
    if (lower.includes("planning edit strategy") || lower.includes("routing to planner model")) {
        return "Planning the edit";
    }
    if (lower.includes("planner selected edit targets")) {
        return "Chose edit targets";
    }
    if (lower.includes("ranking likely target files")) {
        return "Ranking candidate files";
    }
    if (lower.includes("building retrieval bundle")) {
        return "Assembling retrieval context";
    }
    if (lower.includes("consulting mcp bridge")) {
        return "Consulting MCP tools";
    }
    if (lower.includes("running bounded mcp retrieval loop")) {
        return "Running bounded MCP retrieval";
    }
    if (lower.includes("completed bounded mcp retrieval loop")) {
        return "Bounded MCP retrieval complete";
    }
    if (lower.includes("refining mcp retrieval focus")) {
        return "Refining MCP retrieval";
    }
    if (lower.includes("running bounded mcp context loop")) {
        return "Running bounded MCP context";
    }
    if (lower.includes("completed bounded mcp context loop")) {
        return "Bounded MCP context complete";
    }
    if (lower.includes("refining mcp context focus")) {
        return "Refining MCP context";
    }
    if (lower.includes("generating bounded helper script")) {
        return "Generating a bounded helper";
    }
    if (lower.includes("running bounded helper script")) {
        return "Running the bounded helper";
    }
    if (lower.includes("helper script produced a valid patch")) {
        return "Helper generated a valid patch";
    }
    if (lower.includes("helper script fell back")) {
        return "Falling back to direct patch generation";
    }
    if (lower.includes("deterministic")) {
        return "Using a fast local edit path";
    }
    if (lower.includes("routing to model") || lower.includes("generating patch artifact")) {
        return "Handing off to the model";
    }
    if (lower.includes("repairing invalid patch artifact")) {
        return "Repairing the patch artifact";
    }
    if (lower.includes("re-checking repaired patch")) {
        return "Re-validating the repaired patch";
    }
    if (lower.includes("streaming from")) {
        return "Generating a response";
    }
    if (lower.includes("prepared a validated patch artifact")) {
        return "Prepared a validated edit";
    }
    if (lower.includes("applying")) {
        return "Applying the change";
    }
    if (lower.includes("validating")) {
        return "Validating the result";
    }
    return normalizedMode === "agent" ? "Working through the request" : "Thinking";
}
function describeRequestEvent(label, metadata = {}) {
    const raw = String(label || "").trim();
    const lower = raw.toLowerCase();
    if (!raw) {
        return "";
    }
    if (lower.includes("using deterministic scaffold primitive")) {
        const name = metadata.package_name ? String(metadata.package_name) : "";
        const root = metadata.package_root ? String(metadata.package_root) : "";
        if (name && root) {
            return `Scaffolding ${name} in ${root}`;
        }
        if (name) {
            return `Scaffolding ${name}`;
        }
    }
    if (lower.includes("collected candidate files") && metadata.candidate_count) {
        return `Collected ${metadata.candidate_count} candidate files`;
    }
    if (lower.includes("routing to planner model") && metadata.model) {
        return `Planner model: ${metadata.model}`;
    }
    if (lower.includes("generating patch artifact") && metadata.model) {
        return `Generating patch with ${metadata.model}`;
    }
    if (lower.includes("running bounded mcp retrieval loop")) {
        const budget = metadata.step_budget ? ` (${metadata.step_budget} steps)` : "";
        return `Running bounded MCP retrieval${budget}`;
    }
    if (lower.includes("refining mcp retrieval focus") && metadata.refined_term_count) {
        return `Refining retrieval focus (${metadata.refined_term_count} terms)`;
    }
    return raw;
}
function buildRequestErrorStatus(message) {
    const lower = String(message || "").toLowerCase();
    if (lower.includes("ollama")) {
        return "Error talking to Ollama";
    }
    if (lower.includes("patch") || lower.includes("edit")) {
        return "Edit request failed";
    }
    return "DorkPipe request failed";
}
function normalizeStoredChatState(raw) {
    try {
        const base = createInitialChatState();
        if (!raw || !Array.isArray(raw.sessions) || raw.sessions.length === 0) {
            return base;
        }
        const modelStore = normalizeModelStore(raw.modelStore || base.modelStore);
        const reasoningTemplates = normalizeReasoningTemplates(raw.reasoningTemplates, modelStore);
        const sessions = raw.sessions
            .map((session) => ({
            id: String(session?.id || makeId("chat")),
            title: summarizeSessionTitle(session?.title || "New chat"),
            createdAt: String(session?.createdAt || nowIso()),
            updatedAt: String(session?.updatedAt || session?.createdAt || nowIso()),
            messages: Array.isArray(session?.messages) ? session.messages.map(sanitizeMessage).slice(-MAX_HISTORY_MESSAGES * 3) : [],
        }))
            .slice(0, MAX_SAVED_SESSIONS);
        const activeSession = sessions.find((session) => session.id === raw.activeSessionId) || sessions[0];
        return {
            activeSessionId: activeSession.id,
            sessions,
            composerMode: ["ask", "agent", "plan"].includes(String(raw.composerMode || "").toLowerCase())
                ? String(raw.composerMode).toLowerCase()
                : "ask",
            autoApplyEdits: !!raw.autoApplyEdits,
            modelProfile: normalizeModelProfile(raw.modelProfile),
            activeTemplateId: templateById(reasoningTemplates, raw.activeTemplateId)?.id || BUILTIN_TEMPLATE_ID,
            reasoningTemplates,
            modelStore,
        };
    }
    catch {
        return createInitialChatState();
    }
}
function sortSessionsByUpdate(sessions) {
    return [...sessions].sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)));
}
function ensureValidChatStore(chatStore) {
    if (!chatStore || !Array.isArray(chatStore.sessions) || chatStore.sessions.length === 0) {
        return createInitialChatState();
    }
    const sessions = chatStore.sessions.filter(Boolean);
    if (!sessions.length) {
        return createInitialChatState();
    }
    const activeSessionId = sessions.some((session) => session.id === chatStore.activeSessionId)
        ? chatStore.activeSessionId
        : sessions[0].id;
    return {
        ...chatStore,
        sessions,
        activeSessionId,
        modelStore: normalizeModelStore(chatStore.modelStore),
        reasoningTemplates: normalizeReasoningTemplates(chatStore.reasoningTemplates, normalizeModelStore(chatStore.modelStore)),
        activeTemplateId: templateById(normalizeReasoningTemplates(chatStore.reasoningTemplates, normalizeModelStore(chatStore.modelStore)), chatStore.activeTemplateId)?.id || BUILTIN_TEMPLATE_ID,
    };
}
function compactPendingActionForStorage(value) {
    const pending = sanitizePendingAction(value);
    if (!pending) {
        return null;
    }
    return {
        kind: pending.kind,
        title: pending.title,
        artifactDir: pending.artifactDir,
        patchPath: pending.patchPath,
        tracePath: pending.tracePath,
        artifactVersion: pending.artifactVersion,
        structuredEditCount: pending.structuredEditCount,
        structuredEditTypes: pending.structuredEditTypes,
        helperScriptPath: pending.helperScriptPath,
        helperScriptPurpose: pending.helperScriptPurpose,
        helperScriptRuntime: pending.helperScriptRuntime,
        targetFiles: pending.targetFiles,
        requestText: pending.requestText,
        mode: pending.mode,
    };
}
function compactMessageForStorage(message) {
    const safe = sanitizeMessage(message);
    return {
        id: safe.id,
        role: safe.role,
        text: clampText(safe.text, 12000),
        format: safe.format,
        createdAt: safe.createdAt,
        pendingAction: compactPendingActionForStorage(safe.pendingAction),
        run: sanitizeRunRecord(safe.run),
        diffPreview: safe.diffPreview ? clampText(safe.diffPreview, 3000) : "",
    };
}
function compactChatStoreForPersistence(chatStore) {
    const safe = ensureValidChatStore(chatStore);
    return {
        activeSessionId: safe.activeSessionId,
        composerMode: ["ask", "agent", "plan"].includes(String(safe.composerMode || "").toLowerCase())
            ? String(safe.composerMode).toLowerCase()
            : "ask",
        autoApplyEdits: !!safe.autoApplyEdits,
        modelProfile: normalizeModelProfile(safe.modelProfile),
        activeTemplateId: templateById(safe.reasoningTemplates, safe.activeTemplateId)?.id || BUILTIN_TEMPLATE_ID,
        modelStore: {
            updatedAt: safe.modelStore?.updatedAt || nowIso(),
            entries: normalizeModelStore(safe.modelStore).entries.map((entry) => ({
                id: entry.id,
                label: entry.label,
                provider: entry.provider,
                model: entry.model,
                builtIn: entry.builtIn,
                locked: entry.locked,
                installState: entry.installState,
                capabilities: entry.capabilities,
                contextWindow: entry.contextWindow,
                notes: entry.notes,
            })),
        },
        reasoningTemplates: normalizeReasoningTemplates(safe.reasoningTemplates, safe.modelStore).map((template) => ({
            ...deepClone(template),
        })),
        sessions: safe.sessions.slice(0, MAX_SAVED_SESSIONS).map((session) => ({
            id: String(session.id || makeId("chat")),
            title: summarizeSessionTitle(session.title || "New chat"),
            createdAt: String(session.createdAt || nowIso()),
            updatedAt: String(session.updatedAt || session.createdAt || nowIso()),
            messages: Array.isArray(session.messages) ? session.messages.slice(-MAX_HISTORY_MESSAGES * 2).map(compactMessageForStorage) : [],
        })),
    };
}
async function resolveContextBundlePath(root) {
    const candidates = [
        path.join(root, "bin", ".dockpipe", "pipeon-context.md"),
        path.join(root, ".dockpipe", "pipeon-context.md"),
    ];
    for (const p of candidates) {
        try {
            await fs.stat(p);
            return p;
        }
        catch {
            // keep trying
        }
    }
    return null;
}
async function readContextBundle(root) {
    const ctxPath = await resolveContextBundlePath(root);
    if (!ctxPath) {
        return { text: "", path: null, mtime: null };
    }
    const stat = await fs.stat(ctxPath);
    return {
        text: clampText(await fs.readFile(ctxPath, "utf8"), MAX_CONTEXT_CHARS),
        path: ctxPath,
        mtime: stat.mtime.toISOString(),
    };
}
function getWorkspaceRoot() {
    return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || null;
}
async function writeTaskFile(root, kind, prompt) {
    const dir = path.join(root, ".dockpipe", "pipeon", "tasks");
    await ensureDir(dir);
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    const file = path.join(dir, `${kind}-${stamp}.md`);
    const body = [
        `# DorkPipe ${kind} task`,
        "",
        `Created: ${new Date().toISOString()}`,
        "",
        "## Request",
        "",
        prompt,
        "",
        "## Suggested flow",
        "",
        "1. Inspect the relevant files and current behavior.",
        "2. Make the smallest useful code change.",
        "3. Run the relevant validation workflow or tests.",
        "4. Summarize the diff, logs, and any follow-up risk.",
        "",
    ].join("\n");
    await fs.writeFile(file, body, "utf8");
    return file;
}
function buildOllamaUrl(host) {
    return new URL("/api/chat", host.endsWith("/") ? host : `${host}/`);
}
function escapeHtml(text) {
    return String(text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
}
function renderInlineMarkdown(text) {
    const placeholders = [];
    let html = escapeHtml(text);
    html = html.replace(/`([^`]+)`/g, (_match, code) => {
        const token = `@@CODE${placeholders.length}@@`;
        placeholders.push(`<code>${escapeHtml(code)}</code>`);
        return token;
    });
    html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (_match, label, href) => {
        return `<a href="${escapeHtml(href)}">${escapeHtml(label)}</a>`;
    });
    html = html.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    html = html.replace(/\*([^*]+)\*/g, "<em>$1</em>");
    for (let i = 0; i < placeholders.length; i += 1) {
        html = html.replace(`@@CODE${i}@@`, placeholders[i]);
    }
    return html;
}
function renderMarkdown(text) {
    const lines = String(text || "").replace(/\r\n/g, "\n").split("\n");
    const out = [];
    let paragraph = [];
    let listItems = [];
    let inCode = false;
    let codeFence = "";
    let codeLang = "";
    let codeLines = [];
    let quoteLines = [];
    const flushParagraph = () => {
        if (!paragraph.length)
            return;
        out.push(`<p>${renderInlineMarkdown(paragraph.join(" "))}</p>`);
        paragraph = [];
    };
    const flushList = () => {
        if (!listItems.length)
            return;
        out.push(`<ul>${listItems.map((item) => `<li>${renderInlineMarkdown(item)}</li>`).join("")}</ul>`);
        listItems = [];
    };
    const flushQuote = () => {
        if (!quoteLines.length)
            return;
        out.push(`<blockquote>${renderMarkdown(quoteLines.join("\n"))}</blockquote>`);
        quoteLines = [];
    };
    const flushCode = () => {
        const langClass = codeLang ? ` class="language-${escapeHtml(codeLang)}"` : "";
        out.push(`<pre><code${langClass}>${escapeHtml(codeLines.join("\n"))}</code></pre>`);
        codeLines = [];
        codeLang = "";
        codeFence = "";
    };
    for (const line of lines) {
        if (inCode) {
            if (line.startsWith(codeFence)) {
                inCode = false;
                flushCode();
            }
            else {
                codeLines.push(line);
            }
            continue;
        }
        const codeMatch = line.match(/^(```+|~~~+)\s*(\S+)?\s*$/);
        if (codeMatch) {
            flushParagraph();
            flushList();
            flushQuote();
            inCode = true;
            codeFence = codeMatch[1];
            codeLang = codeMatch[2] || "";
            continue;
        }
        if (/^\s*> ?/.test(line)) {
            flushParagraph();
            flushList();
            quoteLines.push(line.replace(/^\s*> ?/, ""));
            continue;
        }
        flushQuote();
        if (/^\s*$/.test(line)) {
            flushParagraph();
            flushList();
            continue;
        }
        const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
        if (headingMatch) {
            flushParagraph();
            flushList();
            const level = headingMatch[1].length;
            out.push(`<h${level}>${renderInlineMarkdown(headingMatch[2])}</h${level}>`);
            continue;
        }
        if (/^---+$/.test(line.trim()) || /^___+$/.test(line.trim())) {
            flushParagraph();
            flushList();
            out.push("<hr />");
            continue;
        }
        const listMatch = line.match(/^\s*[-*]\s+(.+)$/);
        if (listMatch) {
            flushParagraph();
            listItems.push(listMatch[1]);
            continue;
        }
        paragraph.push(line.trim());
    }
    flushParagraph();
    flushList();
    flushQuote();
    if (inCode) {
        flushCode();
    }
    return out.join("");
}
function renderMessageBody(message) {
    if (message.format === "plain") {
        return `<pre class="plain">${escapeHtml(message.text)}</pre>`;
    }
    return renderMarkdown(message.text);
}
function safeRenderMessageBody(message, channel) {
    try {
        return renderMessageBody(message);
    }
    catch (error) {
        const detail = error instanceof Error ? (error.stack || error.message) : String(error);
        if (channel) {
            channel.error(`Failed to render message body: ${detail}`);
        }
        const fallback = message?.text ? String(message.text) : "(Message render failed.)";
        return `<pre class="plain">${escapeHtml(fallback)}</pre>`;
    }
}
function serializeForInlineScript(value) {
    return JSON.stringify(value)
        .replace(/</g, "\\u003c")
        .replace(/\u2028/g, "\\u2028")
        .replace(/\u2029/g, "\\u2029");
}
function logChannelInfo(channel, message, data) {
    if (!channel) {
        return;
    }
    const suffix = data === undefined ? "" : ` ${JSON.stringify(data)}`;
    channel.info(`${message}${suffix}`);
}
async function collectWorkspaceSignals(root) {
    const editor = vscode.window.activeTextEditor;
    const activePath = editor?.document?.uri?.scheme === "file" ? editor.document.uri.fsPath : null;
    const selectionText = editor && !editor.selection.isEmpty
        ? clampText(editor.document.getText(editor.selection), MAX_SELECTION_CHARS)
        : "";
    const openFiles = [];
    for (const group of vscode.window.tabGroups?.all || []) {
        for (const tab of group.tabs || []) {
            const input = tab.input;
            /** @type {vscode.Uri | null} */
            const uri = input && typeof input === "object" && "uri" in input
                ? /** @type {vscode.Uri} */ (input.uri)
                : null;
            if (uri?.scheme === "file" && uri.fsPath) {
                openFiles.push(relativeToRoot(root, uri.fsPath));
            }
        }
    }
    return {
        rootName: path.basename(root),
        activeFile: activePath ? relativeToRoot(root, activePath) : null,
        languageId: editor?.document?.languageId || null,
        selectionText,
        openFiles: [...new Set(openFiles)].slice(0, 8).filter(Boolean),
    };
}
function buildSystemPrompt(signals) {
    return [
        "You are DorkPipe, a local-first repo-aware IDE assistant inside VS Code.",
        "Ground your answers in the active file, selected text, recent conversation, and explicit repo paths from the user request.",
        "Treat scan artifacts, user-guidance artifacts, and compatibility snapshots as secondary context that only matters when the request is clearly about them.",
        "If you provide code, use fenced code blocks with a language tag when possible.",
        "Prefer concise, practical guidance and state uncertainty plainly.",
        signals.activeFile ? `Active file: ${signals.activeFile}` : "Active file: none",
        signals.selectionText ? `Selected text:\n${signals.selectionText}` : "Selected text: none",
        signals.openFiles.length ? `Open files:\n- ${signals.openFiles.join("\n- ")}` : "Open files: none",
        "Compatibility snapshots such as pipeon-context are optional and should not dominate reasoning.",
    ].join("\n\n");
}
function buildConversationMessages(session, signals) {
    const system = { role: "system", content: buildSystemPrompt(signals) };
    const history = session.messages
        .filter((message) => (message.role === "user" || message.role === "assistant") && String(message.text || "").trim())
        .slice(-MAX_HISTORY_MESSAGES)
        .map((message) => ({
        role: message.role,
        content: clampText(message.text, 6000),
    }));
    return [system, ...history];
}
function ollamaChat({ host, model, messages, numCtx }) {
    return new Promise((resolve, reject) => {
        let url;
        try {
            url = buildOllamaUrl(host);
        }
        catch {
            reject(new Error(`Invalid OLLAMA_HOST: ${host}`));
            return;
        }
        const payload = JSON.stringify({
            model,
            stream: false,
            messages,
            options: numCtx ? { num_ctx: numCtx } : undefined,
        });
        const transport = url.protocol === "https:" ? https : http;
        const req = transport.request(url, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Content-Length": Buffer.byteLength(payload),
            },
        }, (res) => {
            let body = "";
            res.setEncoding("utf8");
            res.on("data", (chunk) => {
                body += chunk;
            });
            res.on("end", () => {
                if (res.statusCode && res.statusCode >= 400) {
                    reject(new Error(`Ollama returned HTTP ${res.statusCode}: ${body}`));
                    return;
                }
                try {
                    const parsed = JSON.parse(body);
                    resolve(parsed.message?.content || parsed.response || "");
                }
                catch {
                    reject(new Error(`Could not parse Ollama response: ${body}`));
                }
            });
        });
        req.on("error", (err) => reject(err));
        req.write(payload);
        req.end();
    });
}
function ollamaChatStream({ host, model, messages, onToken, numCtx }) {
    return new Promise((resolve, reject) => {
        let url;
        try {
            url = buildOllamaUrl(host);
        }
        catch {
            reject(new Error(`Invalid OLLAMA_HOST: ${host}`));
            return;
        }
        const payload = JSON.stringify({
            model,
            stream: true,
            messages,
            options: numCtx ? { num_ctx: numCtx } : undefined,
        });
        const transport = url.protocol === "https:" ? https : http;
        let buffer = "";
        let fullText = "";
        const req = transport.request(url, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Content-Length": Buffer.byteLength(payload),
            },
        }, (res) => {
            if (res.statusCode && res.statusCode >= 400) {
                let body = "";
                res.setEncoding("utf8");
                res.on("data", (chunk) => {
                    body += chunk;
                });
                res.on("end", () => reject(new Error(`Ollama returned HTTP ${res.statusCode}: ${body}`)));
                return;
            }
            res.setEncoding("utf8");
            res.on("data", (chunk) => {
                buffer += chunk;
                const lines = buffer.split("\n");
                buffer = lines.pop() || "";
                for (const line of lines) {
                    const trimmed = line.trim();
                    if (!trimmed)
                        continue;
                    try {
                        const parsed = JSON.parse(trimmed);
                        const piece = parsed.message?.content || parsed.response || "";
                        if (piece) {
                            fullText += piece;
                            onToken(piece, fullText);
                        }
                        if (parsed.done) {
                            resolve(fullText);
                            return;
                        }
                    }
                    catch {
                        // ignore partial lines
                    }
                }
            });
            res.on("end", () => resolve(fullText));
        });
        req.on("error", (err) => reject(err));
        req.write(payload);
        req.end();
    });
}
function runCommand(command, cwd, extraEnv = {}) {
    return new Promise((resolve) => {
        cp.exec(command, { cwd, env: { ...process.env, ...extraEnv }, maxBuffer: 1024 * 1024 }, (error, stdout, stderr) => {
            resolve({
                ok: !error,
                code: error && typeof error.code === "number" ? error.code : 0,
                stdout: stdout || "",
                stderr: stderr || "",
            });
        });
    });
}
function fileExists(target) {
    return fs
        .stat(target)
        .then(() => true)
        .catch(() => false);
}
function spawnStreamingCommand(command, args, options = {}) {
    return new Promise((resolve, reject) => {
        const child = cp.spawn(command, args, {
            cwd: options.cwd,
            env: { ...process.env, ...(options.env || {}) },
            stdio: ["ignore", "pipe", "pipe"],
        });
        let stdoutBuffer = "";
        let stderrBuffer = "";
        let stdoutText = "";
        let stderrText = "";
        child.stdout.on("data", (chunk) => {
            const text = String(chunk);
            stdoutText += text;
            stdoutBuffer += text;
            const lines = stdoutBuffer.split("\n");
            stdoutBuffer = lines.pop() || "";
            for (const line of lines) {
                if (options.onStdoutLine) {
                    options.onStdoutLine(line);
                }
            }
        });
        child.stderr.on("data", (chunk) => {
            const text = String(chunk);
            stderrText += text;
            stderrBuffer += text;
            const lines = stderrBuffer.split("\n");
            stderrBuffer = lines.pop() || "";
            for (const line of lines) {
                if (options.onStderrLine) {
                    options.onStderrLine(line);
                }
            }
        });
        child.on("error", (error) => reject(error));
        child.on("close", (code) => {
            if (stdoutBuffer && options.onStdoutLine) {
                options.onStdoutLine(stdoutBuffer);
            }
            if (stderrBuffer && options.onStderrLine) {
                options.onStderrLine(stderrBuffer);
            }
            resolve({
                code: typeof code === "number" ? code : 0,
                stdout: stdoutText,
                stderr: stderrText,
            });
        });
    });
}
async function resolveDorkpipeInvocation(root) {
    const binary = path.join(root, "packages", "dorkpipe", "bin", "dorkpipe");
    if (await fileExists(binary)) {
        const probe = await runCommand(`${shellQuote(binary)} --help`, root);
        const helpText = `${probe.stdout}\n${probe.stderr}`;
        if (probe.ok && /\brequest\b/.test(helpText) && /\bapply-edit\b/.test(helpText)) {
            return {
                command: binary,
                argsPrefix: [],
                cwd: root,
                mode: "binary",
            };
        }
    }
    return {
        command: "go",
        argsPrefix: ["run", "./cmd/dorkpipe"],
        cwd: path.join(root, "packages", "dorkpipe", "lib"),
        mode: "go-run",
    };
}
async function executeNaturalLanguageRequest(root, text, signals, options = {}) {
    const invocation = await resolveDorkpipeInvocation(root);
    const requestMode = ["ask", "agent", "plan"].includes(String(options.mode || "").toLowerCase())
        ? String(options.mode).toLowerCase()
        : "ask";
    const modelProfile = normalizeModelProfile(options.modelProfile);
    const numCtx = resolveNumCtxForProfile(modelProfile);
    const args = [
        ...invocation.argsPrefix,
        "request",
        "--execute",
        "--workdir",
        root,
        "--mode",
        requestMode,
        "--message",
        text,
        "--num-ctx",
        String(numCtx),
    ];
    if (signals.activeFile) {
        args.push("--active-file", signals.activeFile);
    }
    for (const openFile of signals.openFiles || []) {
        args.push("--open-file", openFile);
    }
    if (signals.selectionText) {
        args.push("--selection-text", signals.selectionText);
    }
    /** @type {any} */
    let finalEvent = null;
    /** @type {Record<string, any> | null} */
    let readyToApply = null;
    const stderrLines = [];
    let streamedText = "";
    const result = await spawnStreamingCommand(invocation.command, args, {
        cwd: invocation.cwd,
        env: {
            DOCKPIPE_WORKDIR: root,
        },
        onStdoutLine: (line) => {
            const trimmed = line.trim();
            if (!trimmed)
                return;
            try {
                const event = JSON.parse(trimmed);
                if (options.onEventDetail) {
                    options.onEventDetail(event);
                }
                else if (event.display_text && options.onEvent) {
                    options.onEvent(event.display_text);
                }
                if (event.type === "model_stream") {
                    const piece = event.metadata?.text || "";
                    if (piece) {
                        streamedText += piece;
                        options.onToken?.(piece, streamedText);
                    }
                }
                if (event.type === "ready_to_apply") {
                    readyToApply = event.metadata || {};
                }
                if (event.type === "done" || event.type === "error") {
                    finalEvent = event;
                }
            }
            catch {
                if (options.onEvent) {
                    options.onEvent(trimmed);
                }
            }
        },
        onStderrLine: (line) => {
            const trimmed = line.trim();
            if (!trimmed)
                return;
            stderrLines.push(trimmed);
        },
    });
    if (stderrLines.length) {
        options.channel?.appendLine(stderrLines.join("\n"));
    }
    if (finalEvent?.type === "error") {
        throw new Error(finalEvent.error?.user_message || "DorkPipe request failed.");
    }
    if (result.code !== 0) {
        throw new Error(stderrLines.join("\n") || "DorkPipe request failed.");
    }
    if (!finalEvent) {
        throw new Error("DorkPipe request did not return a final event.");
    }
    return {
        kind: finalEvent.metadata?.route || "chat",
        text: finalEvent.user_message || streamedText || "(No response text returned.)",
        format: "markdown",
        readyToApply,
        metadata: finalEvent.metadata || {},
        status: buildDorkpipeStatus({ ...(finalEvent.metadata || {}), model_profile: modelProfile, num_ctx: numCtx }, requestMode),
    };
}
function buildDorkpipeStatus(metadata, fallbackMode = "ask") {
    const route = metadata.route || "request";
    const mode = metadata.mode || fallbackMode;
    const profile = modelProfileLabel(metadata.model_profile || "balanced");
    const mcpPart = metadata.mcp_connected
        ? `MCP: connected${metadata.mcp_tool_count ? ` (${metadata.mcp_tool_count} tools)` : ""}`
        : "";
    const mcpLoopPart = metadata.mcp_steps_used
        ? `MCP steps: ${metadata.mcp_steps_used}${metadata.mcp_files_read ? `, files ${metadata.mcp_files_read}` : ""}${metadata.mcp_search_hits ? `, hits ${metadata.mcp_search_hits}` : ""}`
        : "";
    if (route === "chat") {
        const parts = [
            `Mode: ${String(mode).replace(/^./, (c) => c.toUpperCase())}`,
            `Model: ${metadata.model || process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL}`,
            `Profile: ${profile}`,
            `Ollama: ${process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST}`,
        ];
        if (metadata.num_ctx) {
            parts.push(`Context: ${metadata.num_ctx}`);
        }
        if (metadata.context_path) {
            parts.push(`Context: ${metadata.context_path}`);
        }
        if (metadata.active_file) {
            parts.push(`Active file: ${metadata.active_file}`);
        }
        if (mcpPart) {
            parts.push(mcpPart);
        }
        if (mcpLoopPart) {
            parts.push(mcpLoopPart);
        }
        return parts.join("  |  ");
    }
    if (route === "inspect") {
        const parts = [
            `Mode: ${String(mode).replace(/^./, (c) => c.toUpperCase())}`,
            `Action: ${metadata.action || "inspect"}`,
        ];
        if (metadata.workflow) {
            parts.push(`Workflow: ${metadata.workflow}`);
        }
        if (metadata.target) {
            parts.push(`Target: ${metadata.target}`);
        }
        if (metadata.context_path) {
            parts.push(`Context: ${metadata.context_path}`);
        }
        if (metadata.validation_status) {
            parts.push(`Validation: ${metadata.validation_status}`);
        }
        if (mcpPart) {
            parts.push(mcpPart);
        }
        if (mcpLoopPart) {
            parts.push(mcpLoopPart);
        }
        return parts.join("  |  ");
    }
    if (route === "edit") {
        const parts = [`Mode: ${String(mode).replace(/^./, (c) => c.toUpperCase())}`, "Route: edit", `Profile: ${profile}`];
        if (metadata.num_ctx) {
            parts.push(`Context: ${metadata.num_ctx}`);
        }
        if (typeof metadata.files_touched === "number") {
            parts.push(`Files: ${metadata.files_touched}`);
        }
        if (metadata.helper_script_used) {
            parts.push("Primitive: sidecar");
        }
        if (metadata.validation_status) {
            parts.push(`Validation: ${metadata.validation_status}`);
        }
        if (metadata.artifact_dir) {
            parts.push(`Artifact: ${metadata.artifact_dir}`);
        }
        if (mcpPart) {
            parts.push(mcpPart);
        }
        if (mcpLoopPart) {
            parts.push(mcpLoopPart);
        }
        return parts.join("  |  ");
    }
    return "Request handled by DorkPipe";
}
async function applyPreparedEdit(root, artifactDir, options = {}) {
    const invocation = await resolveDorkpipeInvocation(root);
    const args = [
        ...invocation.argsPrefix,
        "apply-edit",
        "--workdir",
        root,
        "--artifact-dir",
        artifactDir,
    ];
    /** @type {any} */
    let finalEvent = null;
    const stderrLines = [];
    const result = await spawnStreamingCommand(invocation.command, args, {
        cwd: invocation.cwd,
        env: {
            DOCKPIPE_WORKDIR: root,
        },
        onStdoutLine: (line) => {
            const trimmed = line.trim();
            if (!trimmed)
                return;
            try {
                const event = JSON.parse(trimmed);
                if (event.display_text && options.onEvent) {
                    options.onEvent(event.display_text);
                }
                if (event.type === "done" || event.type === "error") {
                    finalEvent = event;
                }
            }
            catch {
                if (options.onEvent) {
                    options.onEvent(trimmed);
                }
            }
        },
        onStderrLine: (line) => {
            const trimmed = line.trim();
            if (!trimmed)
                return;
            stderrLines.push(trimmed);
        },
    });
    if (stderrLines.length) {
        options.channel?.appendLine(stderrLines.join("\n"));
    }
    if (finalEvent?.type === "error") {
        throw new Error(finalEvent.error?.user_message || "Apply edit failed.");
    }
    if (result.code !== 0) {
        throw new Error(stderrLines.join("\n") || "Apply edit failed.");
    }
    return {
        kind: "edit",
        text: finalEvent?.user_message || "Applied prepared edit.",
        format: "markdown",
        status: "Structured edit applied",
        metadata: finalEvent?.metadata || {},
    };
}
async function readPatchPreview(root, readyToApply) {
    const relPatch = String(readyToApply?.patch_path || "").trim();
    const relArtifact = String(readyToApply?.artifact_dir || "").trim();
    const candidates = [];
    if (relPatch) {
        candidates.push(path.isAbsolute(relPatch) ? relPatch : path.join(root, relPatch));
    }
    if (relArtifact) {
        candidates.push(path.join(root, relArtifact, "patch.diff"));
    }
    for (const candidate of candidates) {
        try {
            const text = await fs.readFile(candidate, "utf8");
            const lines = text.split(/\r?\n/).slice(0, 120).join("\n");
            return clampText(lines, 12000);
        }
        catch {
            // Try next candidate.
        }
    }
    return "";
}
async function readHelperScriptPreview(root, readyToApply) {
    const relHelper = String(readyToApply?.helper_script_path || "").trim();
    if (!relHelper) {
        return "";
    }
    const target = path.isAbsolute(relHelper) ? relHelper : path.join(root, relHelper);
    try {
        const text = await fs.readFile(target, "utf8");
        return clampText(text.split(/\r?\n/).slice(0, 120).join("\n"), 12000);
    }
    catch {
        return "";
    }
}
async function readJsonIfPresent(target) {
    try {
        const text = await fs.readFile(target, "utf8");
        return JSON.parse(text);
    }
    catch {
        return null;
    }
}
async function readTextIfPresent(target, limit = 4000) {
    try {
        const text = await fs.readFile(target, "utf8");
        return clampText(text, limit);
    }
    catch {
        return "";
    }
}
async function readTraceEvents(tracePath) {
    if (!tracePath) {
        return [];
    }
    try {
        const text = await fs.readFile(tracePath, "utf8");
        return text
            .split(/\r?\n/)
            .map((line) => line.trim())
            .filter(Boolean)
            .map((line) => {
            try {
                return JSON.parse(line);
            }
            catch {
                return null;
            }
        })
            .filter(Boolean)
            .slice(-80)
            .map((entry) => ({
            requestId: entry.request_id ? String(entry.request_id) : "",
            parentRequestId: entry.parent_request_id ? String(entry.parent_request_id) : "",
            phase: entry.phase ? String(entry.phase) : "",
            eventType: entry.event_type ? String(entry.event_type) : "",
            label: entry.label ? String(entry.label) : "",
            status: entry.status ? String(entry.status) : "",
            progress: Number.isFinite(Number(entry.progress)) ? Number(entry.progress) : 0,
            metadata: entry.metadata && typeof entry.metadata === "object" ? entry.metadata : null,
            timestamp: entry.timestamp ? String(entry.timestamp) : "",
        }));
    }
    catch {
        return [];
    }
}
function summarizeStructuredEditForRun(item) {
    if (!item || typeof item !== "object") {
        return null;
    }
    return {
        id: item.id ? String(item.id) : "",
        op: item.op ? String(item.op) : "",
        language: item.language ? String(item.language) : "",
        targetFile: item.target_file ? String(item.target_file) : "",
        description: item.description ? String(item.description) : "",
        target: item.target && typeof item.target === "object" ? {
            kind: item.target.kind ? String(item.target.kind) : "",
            symbolName: item.target.symbol_name ? String(item.target.symbol_name) : "",
            symbolKind: item.target.symbol_kind ? String(item.target.symbol_kind) : "",
        } : null,
        range: item.range && typeof item.range === "object" ? {
            startLine: Number.isFinite(Number(item.range.start_line)) ? Number(item.range.start_line) : 0,
            oldLineCount: Number.isFinite(Number(item.range.old_line_count)) ? Number(item.range.old_line_count) : 0,
            newLineCount: Number.isFinite(Number(item.range.new_line_count)) ? Number(item.range.new_line_count) : 0,
        } : null,
        preconditions: Array.isArray(item.preconditions) ? item.preconditions.map((entry) => String(entry)).slice(0, 4) : [],
        postconditions: Array.isArray(item.postconditions) ? item.postconditions.map((entry) => String(entry)).slice(0, 4) : [],
        fallbackNotes: Array.isArray(item.fallback_notes) ? item.fallback_notes.map((entry) => String(entry)).slice(0, 4) : [],
    };
}
async function readRunInspectionData(root, ref, options = {}) {
    const relArtifact = String(ref?.artifact_dir || ref?.artifactDir || "").trim();
    if (!relArtifact) {
        return null;
    }
    const artifactDir = path.isAbsolute(relArtifact) ? relArtifact : path.join(root, relArtifact);
    const relPatch = String(ref?.patch_path || ref?.patchPath || "").trim();
    const patchPath = relPatch ? (path.isAbsolute(relPatch) ? relPatch : path.join(root, relPatch)) : path.join(artifactDir, "patch.diff");
    const relTrace = String(ref?.trace_path || ref?.tracePath || "").trim();
    const tracePath = relTrace ? (path.isAbsolute(relTrace) ? relTrace : path.join(root, relTrace)) : path.join(artifactDir, "trace.jsonl");
    const artifact = await readJsonIfPresent(path.join(artifactDir, "artifact.json"));
    const reasoningArtifact = await readJsonIfPresent(path.join(artifactDir, "reasoning.json"));
    const traceEvents = await readTraceEvents(tracePath);
    const applyLog = await readTextIfPresent(path.join(artifactDir, "apply.log"));
    const validationLog = await readTextIfPresent(path.join(artifactDir, "post-apply-validation.log"));
    const structuredEdits = Array.isArray(artifact?.structured_edits)
        ? artifact.structured_edits.map(summarizeStructuredEditForRun).filter(Boolean).slice(0, 32)
        : [];
    const structuredEditTypes = [...new Set(structuredEdits.map((item) => item.op).filter(Boolean))];
    const tailEvent = traceEvents[traceEvents.length - 1] || null;
    const validationStatus = options.validationStatus || tailEvent?.metadata?.validation_status || "";
    const applyMode = options.applyMode || tailEvent?.metadata?.apply_mode || "";
    const retrievalCandidates = Array.isArray(reasoningArtifact?.retrieval?.candidates) ? reasoningArtifact.retrieval.candidates : [];
    const evidenceNodes = Array.isArray(reasoningArtifact?.evidence?.nodes) ? reasoningArtifact.evidence.nodes : [];
    const evidenceEdges = Array.isArray(reasoningArtifact?.evidence?.edges) ? reasoningArtifact.evidence.edges : [];
    const validationFindings = Array.isArray(reasoningArtifact?.validation?.findings) ? reasoningArtifact.validation.findings : [];
    const citations = Array.isArray(reasoningArtifact?.output?.citations) ? reasoningArtifact.output.citations : [];
    const attempts = Array.isArray(reasoningArtifact?.attempts) ? reasoningArtifact.attempts : [];
    const repairMemory = Array.isArray(reasoningArtifact?.repair_memory) ? reasoningArtifact.repair_memory : [];
    return sanitizeRunRecord({
        artifactDir: relArtifact,
        patchPath: relPatch || path.relative(root, patchPath),
        tracePath: path.relative(root, tracePath),
        artifactVersion: reasoningArtifact?.artifact_version || artifact?.artifact_version || ref?.artifact_version || ref?.artifactVersion || "",
        summary: reasoningArtifact?.output?.summary || artifact?.summary || "",
        state: options.state || (validationStatus ? "applied" : "prepared"),
        validationStatus,
        applyMode,
        targetFiles: Array.isArray(reasoningArtifact?.output?.target_files)
            ? reasoningArtifact.output.target_files
            : (Array.isArray(artifact?.target_files) ? artifact.target_files : (Array.isArray(ref?.target_files) ? ref.target_files : ref?.targetFiles)),
        reasoningKind: reasoningArtifact?.kind || "",
        structuredEditCount: structuredEdits.length,
        structuredEditTypes,
        structuredEdits,
        traceEvents,
        retrievalCandidates,
        evidenceNodes,
        evidenceEdges,
        validationFindings,
        citations,
        attempts,
        decision: reasoningArtifact?.decision || null,
        policy: reasoningArtifact?.policy || null,
        repairMemory,
        applyLog,
        validationLog,
    });
}
function looksLikeShellCommand(text) {
    const trimmed = String(text || "").trim();
    if (!trimmed || /\n/.test(trimmed)) {
        return false;
    }
    const lower = trimmed.toLowerCase();
    return /^(make|npm|pnpm|yarn|bun|go|cargo|git|docker|docker-compose|podman|node|npx|python|python3|bash|sh|cmake|dockpipe|dorkpipe|ollama)\b/.test(lower);
}
function getCliConfirmationRequest(text, mode = "ask") {
    const trimmed = String(text || "").trim();
    const lower = trimmed.toLowerCase();
    let title = "";
    if (lower === "/test") {
        title = "Run the `test` workflow?";
    }
    else if (lower === "/ci") {
        title = "Run the `ci-emulate` workflow?";
    }
    else if (lower.startsWith("/workflow ")) {
        title = `Run ${trimmed.slice("/workflow ".length).trim()}?`;
    }
    else if (lower.startsWith("/validate ")) {
        title = `Validate ${trimmed.slice("/validate ".length).trim()}?`;
    }
    else if (lower.startsWith("/workflow-validate ")) {
        title = `Validate ${trimmed.slice("/workflow-validate ".length).trim()}?`;
    }
    if (!title && looksLikeShellCommand(trimmed)) {
        title = `Run \`${trimmed}\` in a terminal?`;
    }
    if (!title) {
        return null;
    }
    return {
        kind: "command",
        title,
        requestText: trimmed,
        mode,
    };
}
function shouldDelegateSlashToDorkpipe(text) {
    const trimmed = String(text || "").trim().toLowerCase();
    return (trimmed === "/context" ||
        trimmed === "/status" ||
        trimmed === "/bundle" ||
        trimmed.startsWith("/callstack") ||
        trimmed.startsWith("/heap") ||
        trimmed === "/test" ||
        trimmed === "/ci" ||
        trimmed.startsWith("/workflow ") ||
        trimmed.startsWith("/validate ") ||
        trimmed.startsWith("/workflow-validate ") ||
        trimmed.startsWith("/edit "));
}
function launchTerminalCommand(root, name, command, extraEnv = {}) {
    const terminal = vscode.window.createTerminal({
        name,
        cwd: root,
        env: {
            DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
            DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
            ...extraEnv,
        },
    });
    terminal.show(true);
    terminal.sendText(command, true);
}
function summarizeCommandTitle(text) {
    const trimmed = String(text || "").trim();
    if (!trimmed) {
        return "DorkPipe command";
    }
    const parts = trimmed.split(/\s+/).slice(0, 3);
    return `DorkPipe: ${parts.join(" ")}`;
}
async function handleLocalCommand(root, rawText) {
    const text = rawText.trim();
    if (!text.startsWith("/")) {
        return null;
    }
    const parts = text.split(/\s+/);
    const cmd = parts[0].toLowerCase();
    const args = parts.slice(1);
    switch (cmd) {
        case "/help":
            return {
                kind: "local",
                text: [
                    "# DorkPipe local commands",
                    "",
                    "- `/help`",
                    "- `/status`",
                    "- `/bundle`",
                    "- `/context`",
                    "- `/callstack [symbol]`",
                    "- `/heap [pid|profile-path]`",
                    "- `/test`",
                    "- `/ci`",
                    "- `/validate [path]`",
                    "- `/plan <task>`",
                    "- `/edit <task>`",
                    "- `/workflow <name>`",
                    "- `/workflow-validate <path-to-config.yml>`",
                ].join("\n"),
                status: "Local command help",
            };
        case "/status": {
            const result = await runCommand("./packages/pipeon/resolvers/pipeon/bin/pipeon status", root, {
                DOCKPIPE_WORKDIR: root,
                DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
                DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
            });
            return {
                kind: "local",
                text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
                status: "Local status command executed",
            };
        }
        case "/bundle": {
            const result = await runCommand("./packages/pipeon/resolvers/pipeon/bin/pipeon bundle", root, {
                DOCKPIPE_WORKDIR: root,
                DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
                DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
            });
            return {
                kind: "local",
                text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
                status: "Context bundle refreshed",
            };
        }
        case "/context": {
            const ctxPath = await resolveContextBundlePath(root);
            return {
                kind: "local",
                text: ctxPath
                    ? `Legacy compatibility snapshot: \`${path.relative(root, ctxPath)}\``
                    : "No legacy compatibility snapshot found.",
                status: ctxPath ? "Compatibility snapshot available" : "No compatibility snapshot found",
            };
        }
        case "/workflow": {
            if (args.length === 0) {
                return { kind: "local", text: "Usage: `/workflow <name>`", status: "Workflow command needs a name" };
            }
            const workflow = args[0];
            launchTerminalCommand(root, `DorkPipe workflow: ${workflow}`, `./src/bin/dockpipe --workflow ${workflow} --workdir . --`);
            return {
                kind: "local",
                text: `Started workflow \`${workflow}\` in a terminal.`,
                status: `Started workflow ${workflow}`,
            };
        }
        case "/test":
            launchTerminalCommand(root, "DorkPipe test", "./src/bin/dockpipe --workflow test --workdir . --");
            return { kind: "local", text: "Started `test` workflow in a terminal.", status: "Started test workflow" };
        case "/ci":
            launchTerminalCommand(root, "DorkPipe ci-emulate", "./src/bin/dockpipe --workflow ci-emulate --workdir . --");
            return { kind: "local", text: "Started `ci-emulate` workflow in a terminal.", status: "Started ci-emulate workflow" };
        case "/validate": {
            if (args.length > 0) {
                const target = args.join(" ");
                const result = await runCommand(`./src/bin/dockpipe workflow validate ${shellQuote(target)}`, root);
                return {
                    kind: "local",
                    text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
                    status: "Workflow validation finished",
                };
            }
            launchTerminalCommand(root, "DorkPipe validate", "./src/bin/dockpipe --workflow test --workdir . --");
            return { kind: "local", text: "Started validation via the `test` workflow in a terminal.", status: "Started validation workflow" };
        }
        case "/plan": {
            if (args.length === 0) {
                return { kind: "local", text: `Usage: \`${cmd} <task description>\``, status: "Task scaffold command needs a description" };
            }
            const task = args.join(" ");
            const file = await writeTaskFile(root, cmd.slice(1), task);
            const doc = await vscode.workspace.openTextDocument(file);
            await vscode.window.showTextDocument(doc, { preview: false });
            return {
                kind: "local",
                text: `Created ${cmd.slice(1)} task scaffold at \`${path.relative(root, file)}\`.\n\nNext: run \`/test\`, \`/ci\`, or \`/workflow <name>\` after you wire the execution step you want.`,
                status: `Created ${cmd.slice(1)} task scaffold`,
            };
        }
        case "/workflow-validate": {
            if (args.length === 0) {
                return { kind: "local", text: "Usage: `/workflow-validate <path-to-config.yml>`", status: "Workflow validation needs a path" };
            }
            const result = await runCommand(`./src/bin/dockpipe workflow validate ${shellQuote(args.join(" "))}`, root);
            return {
                kind: "local",
                text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
                status: "Workflow validation finished",
            };
        }
        default:
            return {
                kind: "local",
                text: `Unknown DorkPipe local command: \`${cmd}\`\n\nTry \`/help\`.`,
                status: "Unknown local command",
            };
    }
}
async function executeDorkpipeRequest(root, session, text, options = {}) {
    const onToken = typeof options.onToken === "function" ? options.onToken : null;
    const onEvent = typeof options.onEvent === "function" ? options.onEvent : null;
    const mode = ["ask", "agent", "plan"].includes(String(options.mode || "").toLowerCase())
        ? String(options.mode).toLowerCase()
        : "ask";
    const modelProfile = normalizeModelProfile(options.modelProfile);
    const numCtx = resolveNumCtxForProfile(modelProfile);
    const reasoningTemplate = options.reasoningTemplate && typeof options.reasoningTemplate === "object"
        ? options.reasoningTemplate
        : null;
    const emitEvent = (label) => {
        if (onEvent) {
            onEvent(label);
        }
    };
    emitEvent("Received request");
    if (reasoningTemplate?.name) {
        emitEvent(`Reasoning template: ${reasoningTemplate.name}`);
    }
    const signals = await collectWorkspaceSignals(root);
    if (!text.trim().startsWith("/") || shouldDelegateSlashToDorkpipe(text)) {
        return executeNaturalLanguageRequest(root, text, signals, {
            onEvent,
            channel: options.channel,
            onToken,
            mode,
            modelProfile,
            reasoningTemplate,
        });
    }
    const local = await handleLocalCommand(root, text);
    if (local) {
        emitEvent("Routing to safe local action");
        return {
            kind: "local",
            text: local.text,
            format: "markdown",
            status: local.status || "Local DorkPipe command executed",
            contextPath: null,
        };
    }
    emitEvent("Inspecting workspace context");
    if (signals.activeFile) {
        emitEvent(`Active file: ${signals.activeFile}`);
    }
    emitEvent("Routing to model");
    const host = process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST;
    const model = process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL;
    const messages = buildConversationMessages(session, signals);
    let answer = "";
    if (onToken) {
        emitEvent(`Streaming from ${model}`);
        answer = await ollamaChatStream({
            host,
            model,
            messages,
            onToken,
            numCtx,
        });
    }
    else {
        emitEvent(`Waiting for ${model}`);
        answer = await ollamaChat({
            host,
            model,
            messages,
            numCtx,
        });
    }
    return {
        kind: "model",
        text: answer || "(No response text returned.)",
        format: "markdown",
        contextPath: null,
        status: `Template: ${reasoningTemplate?.name || "DockPipe Default"}  |  Model: ${process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL}  |  Profile: ${modelProfileLabel(modelProfile)}  |  num_ctx: ${numCtx}  |  Ollama: ${process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST}  |  Workspace cues: ${signals.activeFile || "repo-wide"}`,
    };
}
function renderChatHtml(webview, state) {
    const nonce = String(Date.now());
    const initialStateJson = serializeForInlineScript(state || {});
    return `<!doctype html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <style>
      :root {
        color-scheme: dark light;
      }
      * { box-sizing: border-box; }
      body {
        font-family: var(--vscode-font-family);
        margin: 0;
        background:
          radial-gradient(circle at top, color-mix(in srgb, var(--vscode-button-background) 7%, transparent) 0%, transparent 34%),
          linear-gradient(180deg, color-mix(in srgb, var(--vscode-sideBar-background) 98%, transparent), var(--vscode-editor-background));
        color: var(--vscode-editor-foreground);
      }
      .wrap {
        display: grid;
        grid-template-rows: auto 1fr auto;
        height: 100vh;
      }
      .header {
        padding: 8px 12px;
        border-bottom: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 96%, transparent);
      }
      .eyebrow {
        display: none;
      }
      .titleRow {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
      }
      .titleBlock {
        display: flex;
        align-items: center;
        gap: 8px;
        min-width: 0;
      }
      .titleIcon {
        color: var(--vscode-descriptionForeground);
        font-size: 14px;
        line-height: 1;
      }
      .title {
        font-size: 13px;
        font-weight: 600;
        margin: 0;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }
      .sub {
        display: none;
      }
      .sessionPicker {
        min-width: 160px;
        max-width: 260px;
      }
      .headerSpacer {
        flex: 1;
      }
      .headerSelect {
        min-width: 170px;
        max-width: 240px;
      }
      .iconBtn {
        min-width: 34px;
        width: 34px;
        height: 34px;
        padding: 0;
        border-radius: 999px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        font-size: 14px;
      }
      .headerHint {
        color: var(--vscode-descriptionForeground);
        font-size: 11px;
      }
      .headerActions {
        display: flex;
        gap: 8px;
        align-items: center;
      }
      select, textarea, button {
        font: inherit;
      }
      select {
        min-width: 120px;
        max-width: 240px;
        background: var(--vscode-dropdown-background, var(--vscode-input-background));
        color: var(--vscode-dropdown-foreground, var(--vscode-input-foreground));
        border: 1px solid var(--vscode-dropdown-border, var(--vscode-input-border));
        border-radius: 10px;
        padding: 8px 10px;
      }
      .btn {
        border: 0;
        border-radius: 10px;
        padding: 9px 13px;
        min-width: 80px;
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        font-weight: 600;
        cursor: pointer;
      }
      .btn.ghost {
        background: color-mix(in srgb, var(--vscode-button-background) 14%, transparent);
        color: var(--vscode-editor-foreground);
        border: 1px solid var(--vscode-panel-border);
      }
      .btn:disabled { opacity: 0.6; cursor: default; }
      .trace {
        display: none;
      }
      .bootSentinel {
        margin: 10px 12px 0;
        padding: 8px 10px;
        border-radius: 10px;
        border: 1px solid rgba(255, 170, 0, 0.35);
        background: rgba(255, 170, 0, 0.08);
        color: #f6d28b;
        font-size: 11px;
      }
      .traceItem {
        padding: 5px 9px;
        border-radius: 999px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 92%, transparent);
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .transcript {
        padding: 10px 12px 18px;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 8px;
      }
      .msg {
        max-width: min(860px, 100%);
        padding: 10px 12px;
        border-radius: 14px;
        border: 1px solid var(--vscode-panel-border);
        box-shadow: none;
      }
      .msg.user {
        background: linear-gradient(180deg, color-mix(in srgb, var(--vscode-button-background) 18%, transparent), color-mix(in srgb, var(--vscode-button-background) 9%, transparent));
        border-bottom-right-radius: 6px;
        align-self: flex-end;
        width: min(860px, 100%);
      }
      .msg.assistant {
        background: linear-gradient(180deg, color-mix(in srgb, var(--vscode-editorWidget-background) 96%, transparent), color-mix(in srgb, var(--vscode-editorWidget-background) 84%, transparent));
        border-bottom-left-radius: 6px;
        align-self: stretch;
      }
      .role {
        font-size: 11px;
        opacity: 0.82;
        margin-bottom: 6px;
        text-transform: uppercase;
        letter-spacing: 0.08em;
      }
      .body {
        line-height: 1.58;
        font-size: 13px;
        word-break: break-word;
      }
      .body p:first-child,
      .body h1:first-child,
      .body h2:first-child,
      .body h3:first-child { margin-top: 0; }
      .body p:last-child,
      .body ul:last-child,
      .body pre:last-child,
      .body blockquote:last-child { margin-bottom: 0; }
      .body pre {
        margin: 12px 0;
        padding: 12px;
        overflow-x: auto;
        background: color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 92%, transparent);
        border: 1px solid var(--vscode-panel-border);
        border-radius: 12px;
      }
      .body code {
        font-family: var(--vscode-editor-font-family, monospace);
        font-size: 0.95em;
      }
      .body :not(pre) > code {
        padding: 2px 6px;
        border-radius: 6px;
        background: color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 68%, transparent);
      }
      .body ul {
        padding-left: 20px;
      }
      .body blockquote {
        margin: 12px 0;
        padding-left: 12px;
        border-left: 3px solid var(--vscode-button-background);
        color: var(--vscode-descriptionForeground);
      }
      .body a {
        color: var(--vscode-textLink-foreground);
      }
      .pendingCard {
        margin-top: 10px;
        padding: 10px;
        border-radius: 12px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 92%, transparent);
      }
      .pendingTitle {
        font-size: 12px;
        font-weight: 700;
        margin-bottom: 8px;
      }
      .pendingMeta {
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
        margin-bottom: 8px;
      }
      .pendingFiles {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin: 0 0 8px;
      }
      .pendingFile {
        padding: 3px 8px;
        border-radius: 999px;
        font-size: 11px;
        border: 1px solid var(--vscode-panel-border);
        color: var(--vscode-descriptionForeground);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 94%, transparent);
      }
      .diffPreview {
        margin: 8px 0 0;
        padding: 10px;
        overflow-x: auto;
        border-radius: 10px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 92%, transparent);
        font-family: var(--vscode-editor-font-family, monospace);
        font-size: 12px;
        line-height: 1.45;
        white-space: pre-wrap;
      }
      .diffFiles {
        display: grid;
        gap: 10px;
        margin-top: 8px;
      }
      .diffFile {
        border: 1px solid var(--vscode-panel-border);
        border-radius: 10px;
        overflow: hidden;
        background: color-mix(in srgb, var(--vscode-sideBar-background) 94%, transparent);
      }
      .diffFileHead {
        display: flex;
        justify-content: space-between;
        gap: 8px;
        padding: 8px 10px;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
        border-bottom: 1px solid var(--vscode-panel-border);
      }
      .diffFileName {
        font-weight: 700;
        color: var(--vscode-foreground);
      }
      .diffLines {
        font-family: var(--vscode-editor-font-family, monospace);
        font-size: 12px;
        line-height: 1.45;
      }
      .diffLine {
        padding: 2px 10px;
        white-space: pre-wrap;
      }
      .diffLine.add {
        color: #8fd19e;
        background: rgba(46, 160, 67, 0.12);
      }
      .diffLine.remove {
        color: #f5a5a5;
        background: rgba(248, 81, 73, 0.12);
      }
      .pendingActions {
        display: flex;
        gap: 8px;
        margin-top: 10px;
      }
      .liveCard {
        margin-top: 8px;
        padding: 10px;
        border-radius: 12px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 94%, transparent);
      }
      .liveRow {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 8px;
      }
      .liveDot {
        width: 10px;
        height: 10px;
        border-radius: 999px;
        background: var(--vscode-button-background);
        box-shadow: 0 0 0 0 color-mix(in srgb, var(--vscode-button-background) 45%, transparent);
        animation: pulse 1.4s ease-in-out infinite;
      }
      .liveTitle {
        font-size: 12px;
        font-weight: 700;
      }
      .liveTrace {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-top: 4px;
      }
      .liveChip {
        padding: 4px 8px;
        border-radius: 999px;
        font-size: 11px;
        border: 1px solid var(--vscode-panel-border);
        color: var(--vscode-descriptionForeground);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 92%, transparent);
      }
      .toggleWrap {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .toggleWrap input {
        margin: 0;
      }
      .plain {
        white-space: pre-wrap;
        margin: 0;
        font-family: var(--vscode-editor-font-family, monospace);
      }
      .composerWrap {
        border-top: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 96%, transparent);
        padding: 10px 12px 12px;
      }
      .status {
        padding: 0 2px 8px;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .composer {
        display: grid;
        grid-template-columns: 1fr;
        gap: 8px;
      }
      textarea {
        resize: vertical;
        min-height: 124px;
        max-height: 280px;
        width: 100%;
        background: color-mix(in srgb, var(--vscode-input-background) 88%, var(--vscode-editor-background));
        color: var(--vscode-input-foreground);
        border: 1px solid var(--vscode-input-border);
        border-radius: 16px;
        padding: 14px 14px;
        line-height: 1.45;
      }
      .actions {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        align-items: flex-end;
      }
      .controlsMeta {
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
      }
      .hint {
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
        max-width: 320px;
        line-height: 1.35;
      }
      .toolRow {
        display: flex;
        gap: 8px;
        align-items: center;
        flex-wrap: wrap;
      }
      .sendWrap {
        display: flex;
        gap: 8px;
        align-items: center;
        flex-wrap: wrap;
      }
      @media (max-width: 520px) {
        .titleRow, .actions {
          flex-direction: column;
          align-items: stretch;
        }
        .headerActions {
          flex-wrap: wrap;
        }
        select {
          max-width: none;
        }
      }
      @keyframes pulse {
        0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--vscode-button-background) 45%, transparent); }
        70% { box-shadow: 0 0 0 9px transparent; }
        100% { box-shadow: 0 0 0 0 transparent; }
      }
    </style>
  </head>
  <body>
    <div class="wrap">
      <header class="header">
        <div class="titleRow">
          <div class="titleBlock">
            <div class="titleIcon">←</div>
            <div class="titleStack">
              <h1 class="title">Workspace Chat</h1>
              <div class="headerMeta" id="headerMeta"></div>
            </div>
          </div>
          <div class="headerSpacer"></div>
          <div class="headerActions">
            <select id="sessionSelect" class="headerSelect" aria-label="Chat session"></select>
            <button class="btn ghost iconBtn" id="settingsBtn" type="button" title="Reasoning templates and models">⚙</button>
            <button class="btn ghost iconBtn" id="clearBtn" type="button" title="Clear chat">↺</button>
            <button class="btn ghost iconBtn" id="newChatBtn" type="button" title="New chat">✎</button>
          </div>
        </div>
        <div class="trace" id="trace"></div>
      </header>
      <div class="bootSentinel" id="bootSentinel">DorkPipe UI is waiting for client-side boot. If this stays visible, the webview script did not start.</div>
      <aside class="settingsDrawer hidden" id="settingsDrawer" aria-hidden="true">
        <div class="settingsHeader">
          <div>
            <div class="settingsEyebrow">DockPipe</div>
            <h2>Workspace Settings</h2>
          </div>
          <button class="btn ghost iconBtn" id="settingsCloseBtn" type="button" title="Close settings">×</button>
        </div>
        <div class="settingsBody">
          <section class="settingsSection settingsOverview">
            <div class="sectionHead">
              <div>
                <h3>Reasoning Surface</h3>
                <p>Use this page as the clean launch point for templates and models. Editing happens in the main studio.</p>
              </div>
            </div>
            <div class="settingsSummaryGrid">
              <article class="summaryCard">
                <div class="summaryLabel">Active template</div>
                <div class="summaryValue" id="activeTemplateSummary">DockPipe Default</div>
              </article>
              <article class="summaryCard">
                <div class="summaryLabel">Templates</div>
                <div class="summaryValue" id="templateCountSummary">0 available</div>
              </article>
              <article class="summaryCard">
                <div class="summaryLabel">Models</div>
                <div class="summaryValue" id="modelCountSummary">0 registered</div>
              </article>
            </div>
          </section>
          <section class="settingsSection">
            <div class="sectionHead">
              <div>
                <h3>Templates</h3>
                <p>Pick a template, switch the active one, or jump into the designer to edit and create.</p>
              </div>
              <div class="toolbarRow compactRow">
                <button class="btn" id="newTemplateBtn" type="button">Create template</button>
              </div>
            </div>
            <select id="templateSelect" class="hidden" aria-label="Active template"></select>
            <div class="templateManagerList" id="templateManagerList"></div>
          </section>
          <section class="settingsSection">
            <div class="sectionHead">
              <div>
                <h3>Models</h3>
                <p>Keep the summary clean here, then open the full model manager when you need to register or prune entries.</p>
              </div>
            </div>
            <div class="modelStoreSummary" id="modelStoreSummary"></div>
            <div class="toolbarRow">
              <button class="btn" id="openModelManagerBtn" type="button">Open model manager</button>
            </div>
          </section>
        </div>
      </aside>
      <main class="transcript" id="transcript"></main>
      <section class="studioSurface hidden" id="studioSurface" aria-hidden="true">
        <div class="studioHeader">
          <div>
            <div class="settingsEyebrow">DockPipe Studio</div>
            <h2 id="studioTitle">Template Designer</h2>
            <div class="canvasHint" id="studioMeta">Inspect and shape the active DockPipe reasoning surface.</div>
          </div>
          <div class="toolbarRow">
            <button class="btn ghost" id="studioBackBtn" type="button">Back to chat</button>
          </div>
        </div>
        <div class="studioScroll">
          <section class="settingsSection" id="templateStudio">
            <div class="sectionHead">
              <div>
                <h3>Visual Designer</h3>
                <p>Edit the active template here. Build the flow visually, then save or duplicate it once the shape feels right.</p>
              </div>
              <div class="toolbarRow compactRow">
                <button class="btn ghost" id="copyTemplateBtn" type="button">Duplicate</button>
                <button class="btn ghost danger" id="deleteTemplateBtn" type="button">Delete</button>
                <button class="btn" id="saveTemplateBtn" type="button">Save template</button>
              </div>
            </div>
            <div class="designerLayout">
              <div class="designerPalette">
                <div class="paletteLabel">Primitives</div>
                <button class="primitiveTile" type="button" draggable="true" data-primitive-type="dockpipe">DockPipe</button>
                <button class="primitiveTile" type="button" draggable="true" data-primitive-type="model">Model</button>
                <button class="primitiveTile" type="button" draggable="true" data-primitive-type="loop">Loop Control</button>
              </div>
              <div class="designerCanvasWrap">
                <div class="canvasToolbar">
                  <span class="canvasLabel">Canvas</span>
                  <span class="canvasHint">Drop a primitive onto the root canvas or into a loop body.</span>
                </div>
                <div class="designerCanvas" id="designerCanvas"></div>
              </div>
              <div class="designerInspector">
                <div class="inspectorHeader">
                  <div class="paletteLabel">Inspector</div>
                  <div class="canvasHint" id="inspectorTarget">Template</div>
                </div>
                <div class="inspectorScroll">
                  <div class="inspectorGroup">
                    <label class="field">
                      <span>Template name</span>
                      <input id="templateNameInput" type="text" />
                    </label>
                    <label class="field">
                      <span>Description</span>
                      <textarea id="templateDescriptionInput" rows="3"></textarea>
                    </label>
                  </div>
                  <div class="inspectorGroup">
                    <h4>Global modifiers</h4>
                    <label class="field">
                      <span>Safety rules</span>
                      <textarea id="templateSafetyRulesInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Output requirements</span>
                      <textarea id="templateOutputRequirementsInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Confidence threshold</span>
                      <input id="templateConfidenceInput" type="number" min="0" max="1" step="0.01" />
                    </label>
                    <label class="field">
                      <span>Execution constraints</span>
                      <textarea id="templateExecutionConstraintsInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Routing preference</span>
                      <input id="templateRoutingPreferenceInput" type="text" />
                    </label>
                  </div>
                  <div class="inspectorGroup">
                    <h4>Decision and guidance</h4>
                    <label class="field">
                      <span>Template orchestration guidance</span>
                      <textarea id="templateOrchestrationGuidanceInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Template model guidance</span>
                      <textarea id="templateModelGuidanceInput" rows="3"></textarea>
                    </label>
                  </div>
                  <div class="inspectorGroup">
                    <h4>Selected node</h4>
                    <label class="field">
                      <span>Label</span>
                      <input id="nodeLabelInput" type="text" />
                    </label>
                    <label class="field">
                      <span>Notes</span>
                      <textarea id="nodeNotesInput" rows="2"></textarea>
                    </label>
                    <label class="field">
                      <span>Decision metadata</span>
                      <textarea id="nodeDecisionInput" rows="2"></textarea>
                    </label>
                    <label class="field">
                      <span>Node orchestration guidance</span>
                      <textarea id="nodeOrchestrationInput" rows="2"></textarea>
                    </label>
                    <label class="field">
                      <span>Node model guidance</span>
                      <textarea id="nodeModelInput" rows="2"></textarea>
                    </label>
                    <label class="field" id="nodeModelField">
                      <span>Model</span>
                      <select id="nodeModelSelect"></select>
                    </label>
                    <label class="field" id="nodeTaskField">
                      <span>Task / phase</span>
                      <input id="nodeTaskInput" type="text" />
                    </label>
                    <label class="field" id="nodeOutputField">
                      <span>Output mode</span>
                      <input id="nodeOutputInput" type="text" />
                    </label>
                    <label class="field" id="nodeLoopIterationsField">
                      <span>Loop iterations</span>
                      <input id="nodeLoopIterationsInput" type="number" min="1" max="8" step="1" />
                    </label>
                    <div class="toolbarRow">
                      <button class="btn ghost danger" id="removeNodeBtn" type="button">Remove node</button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>
          <section class="settingsSection hidden" id="modelStudio">
            <div class="sectionHead">
              <div>
                <h3>Model Store</h3>
                <p>Keep model references explicit and reusable. Register entries here once, then point templates at them by id.</p>
              </div>
            </div>
            <div class="modelStoreSummary" id="modelStoreSummaryStudio"></div>
            <div class="modelStudioLayout">
              <div class="modelStudioListPanel">
                <div class="paletteLabel">Registered models</div>
                <div class="modelStoreList" id="modelStoreList"></div>
              </div>
              <div class="modelEditor">
                <div class="paletteLabel">Register custom model</div>
                <div class="formGrid">
                  <label class="field"><span>Model id</span><input id="modelEntryIdInput" type="text" /></label>
                  <label class="field"><span>Label</span><input id="modelEntryLabelInput" type="text" /></label>
                  <label class="field"><span>Provider</span><input id="modelEntryProviderInput" type="text" value="ollama" /></label>
                  <label class="field"><span>Model name</span><input id="modelEntryModelInput" type="text" /></label>
                  <label class="field"><span>Capabilities</span><input id="modelEntryCapabilitiesInput" type="text" placeholder="chat, edit, reasoning" /></label>
                  <label class="field"><span>Context window</span><input id="modelEntryContextWindowInput" type="number" min="1024" step="1024" value="8192" /></label>
                  <label class="field fieldWide"><span>Notes</span><textarea id="modelEntryNotesInput" rows="2"></textarea></label>
                </div>
                <div class="toolbarRow">
                  <button class="btn" id="saveModelEntryBtn" type="button">Save model</button>
                </div>
              </div>
            </div>
          </section>
          <section class="settingsSection hidden" id="runStudio">
            <div class="sectionHead">
              <div>
                <h3>Run Inspector</h3>
                <p>Inspect the structured edit plan, execution trace, and validation results for a prepared or applied run.</p>
              </div>
            </div>
            <div class="runInspectorBody" id="runInspectorBody">
              <div class="emptyBlock">Pick an assistant run from the chat transcript to inspect it here.</div>
            </div>
          </section>
        </div>
      </section>
      <div class="composerWrap">
        <div class="status" id="status"></div>
        <form class="composer" id="composer">
          <textarea id="prompt" placeholder="Ask DorkPipe about this repo... or use /help for local commands"></textarea>
          <div class="actions">
            <div class="controlsMeta">
              <div class="toolRow">
                <label class="toggleWrap"><input id="autoApplyEdits" type="checkbox" /> Auto</label>
                <select id="modelProfileSelect" aria-label="Model profile">
                  <option value="fast">Fast</option>
                  <option value="balanced">Balanced</option>
                  <option value="deep">Deep</option>
                  <option value="max">Max</option>
                </select>
                <select id="modeSelect" aria-label="Request mode">
                  <option value="ask">Ask</option>
                  <option value="agent">Agent</option>
                  <option value="plan">Plan</option>
                </select>
              </div>
              <div class="hint">Local-first routing with safe primitives, bounded tools, and streamed model steps.</div>
            </div>
            <div class="sendWrap">
              <button class="btn ghost" id="newFromComposer" type="button">New</button>
              <button class="btn" id="send" type="submit">Send</button>
            </div>
          </div>
        </form>
      </div>
    </div>
    <script nonce="${nonce}">
      let vscode = { postMessage() {}, getState() { return null; }, setState() {} };
      if (typeof acquireVsCodeApi === "function") {
        try {
          vscode = acquireVsCodeApi();
        } catch {
          // Keep the stub so the UI can still render a visible error.
        }
      }

      function renderFatalError(message) {
        const safe = String(message || "Unknown webview error");
        document.body.innerHTML = '<div style="padding:16px;font-family:var(--vscode-font-family);color:var(--vscode-editor-foreground);background:var(--vscode-editor-background);height:100vh;box-sizing:border-box;">'
          + '<article style="max-width:760px;border:1px solid var(--vscode-panel-border);border-radius:14px;padding:16px;background:color-mix(in srgb, var(--vscode-editorWidget-background) 94%, transparent);">'
          + '<div style="font-size:11px;opacity:.8;text-transform:uppercase;letter-spacing:.08em;margin-bottom:8px;">DorkPipe</div>'
          + '<h2 style="margin:0 0 10px;font-size:18px;">The chat UI hit a client-side error.</h2>'
          + '<p style="margin:0 0 10px;line-height:1.5;">The panel stayed alive, but the webview renderer failed.</p>'
          + '<pre style="margin:0;padding:12px;border-radius:10px;overflow:auto;background:color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 92%, transparent);border:1px solid var(--vscode-panel-border);white-space:pre-wrap;">'
          + safe.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
          + '</pre></article></div>';
      }

      function reportClientError(kind, error) {
        const message = error instanceof Error ? (error.stack || error.message) : String(error);
        try {
          vscode.postMessage({ type: "clientError", kind, message });
        } catch {
          // Ignore bridge failures while reporting a renderer error.
        }
        renderFatalError(message);
      }

      function postDiag(stage, extra) {
        try {
          vscode.postMessage({ type: "diag", stage, extra: extra || null });
        } catch {
          // Ignore bridge failures while emitting diagnostics.
        }
      }

      postDiag("script-start");

      if (typeof acquireVsCodeApi !== "function") {
        reportClientError("boot", "acquireVsCodeApi is unavailable in this webview");
      }
      postDiag("vscode-api-ready", { hasBridge: typeof acquireVsCodeApi === "function" });

      window.addEventListener("error", (event) => {
        reportClientError("error", event?.error || event?.message || "Unknown webview error");
      });

      window.addEventListener("unhandledrejection", (event) => {
        reportClientError("unhandledrejection", event?.reason || "Unhandled promise rejection");
      });

      const form = document.getElementById("composer");
      const prompt = document.getElementById("prompt");
      const send = document.getElementById("send");
      const clearBtn = document.getElementById("clearBtn");
      const newChatBtn = document.getElementById("newChatBtn");
      const newFromComposer = document.getElementById("newFromComposer");
      const sessionSelect = document.getElementById("sessionSelect");
      const modeSelect = document.getElementById("modeSelect");
      const autoApplyEdits = document.getElementById("autoApplyEdits");
      const modelProfileSelect = document.getElementById("modelProfileSelect");
      const transcript = document.getElementById("transcript");
      const status = document.getElementById("status");
      const trace = document.getElementById("trace");
      const bootSentinel = document.getElementById("bootSentinel");
      if (bootSentinel) {
        bootSentinel.style.display = "none";
      }
      postDiag("dom-ready", {
        hasForm: !!form,
        hasPrompt: !!prompt,
        hasTranscript: !!transcript,
      });
      const initialState = (() => {
        try {
          return JSON.parse(${initialStateJson});
        } catch {
          return {};
        }
      })();
      postDiag("initial-state-parsed", {
        keys: Object.keys(initialState || {}),
        messages: Array.isArray(initialState?.messages) ? initialState.messages.length : 0,
      });
      let viewState = (() => {
        const raw = vscode.getState();
        if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
          return { draft: "", pinnedToBottom: true, mode: "ask", modelProfile: "balanced" };
        }
        return {
          draft: typeof raw.draft === "string" ? raw.draft : "",
          pinnedToBottom: raw.pinnedToBottom !== false,
          mode: ["ask", "agent", "plan"].includes(String(raw.mode || "").toLowerCase()) ? String(raw.mode).toLowerCase() : "ask",
          modelProfile: ["fast", "balanced", "deep", "max"].includes(String(raw.modelProfile || "").toLowerCase())
            ? String(raw.modelProfile).toLowerCase()
            : "balanced",
        };
      })();
      let currentState = initialState && typeof initialState === "object" ? initialState : {};

      function escapeHtml(value) {
        return String(value || "")
          .replace(/&/g, "&amp;")
          .replace(/</g, "&lt;")
          .replace(/>/g, "&gt;")
          .replace(/"/g, "&quot;")
          .replace(/'/g, "&#39;");
      }

      function renderCompactDiff(diffText) {
        const raw = String(diffText || "").trim();
        if (!raw) {
          return "";
        }
        const sections = raw.split(/\n{2,}/).map((block) => block.trim()).filter(Boolean);
        const cards = sections.map((section) => {
          const lines = section.split("\n");
          const header = lines.shift() || "";
          const fileMatch = header.match(/^#\s+(.+?)\s+\+(\d+)\s+\-(\d+)$/);
          const fileName = fileMatch ? fileMatch[1] : header.replace(/^#\s+/, "");
          const adds = fileMatch ? fileMatch[2] : "0";
          const removes = fileMatch ? fileMatch[3] : "0";
          const body = lines
            .map((line) => {
              const cls = line.startsWith("+") ? " add" : line.startsWith("-") ? " remove" : "";
              return '<div class="diffLine' + cls + '">' + escapeHtml(line) + '</div>';
            })
            .join("");
          return [
            '<div class="diffFile">',
            '<div class="diffFileHead"><div class="diffFileName">' + escapeHtml(fileName || "file") + '</div><div>+' + adds + ' -' + removes + '</div></div>',
            '<div class="diffLines">' + body + '</div>',
            '</div>',
          ].join("");
        });
        return '<div class="diffFiles">' + cards.join("") + '</div>';
      }

      function renderPendingAction(message) {
        const pending = message.pendingAction;
        if (!pending) {
          return "";
        }
        const meta = [];
        if (pending.kind === "edit" && Array.isArray(pending.targetFiles) && pending.targetFiles.length) {
          meta.push(pending.targetFiles.length + " file" + (pending.targetFiles.length === 1 ? "" : "s"));
        }
        if (pending.kind === "edit" && pending.helperScriptRuntime) {
          meta.push("Sidecar: " + pending.helperScriptRuntime);
        }
        if (pending.kind === "command" && pending.requestText) {
          meta.push("Command: " + pending.requestText);
        }
        const files = pending.kind === "edit" && Array.isArray(pending.targetFiles) && pending.targetFiles.length
          ? '<div class="pendingFiles">' + pending.targetFiles.slice(0, 3).map((file) => '<div class="pendingFile">' + escapeHtml(file) + '</div>').join("") + '</div>'
          : "";
        const helper = pending.helperScriptPreview
          ? [
              '<div class="pendingMeta">Uses bounded helper' + (pending.helperScriptPurpose ? ": " + escapeHtml(pending.helperScriptPurpose) : "") + "</div>",
            ].join("")
          : "";
        const diff = pending.diffPreview
          ? renderCompactDiff(pending.diffPreview)
          : "";
        return [
          '<div class="pendingCard">',
          '<div class="pendingTitle">' + escapeHtml(pending.title || (pending.kind === "edit" ? "Review edit" : "Confirm command")) + "</div>",
          meta.length ? '<div class="pendingMeta">' + escapeHtml(meta.join("  |  ")) + "</div>" : "",
          files,
          helper,
          diff,
          '<div class="pendingActions">',
          '<button class="btn" type="button" data-pending-action="approve" data-message-id="' + escapeHtml(message.id) + '">' + (pending.kind === "edit" ? "Apply" : "Run") + "</button>",
          '<button class="btn ghost" type="button" data-pending-action="dismiss" data-message-id="' + escapeHtml(message.id) + '">Dismiss</button>',
          "</div>",
          "</div>",
        ].join("");
      }

      function renderDiffPreview(message) {
        if (!message.diffPreview || (message.pendingAction && message.pendingAction.kind === "edit")) {
          return "";
        }
        return [
          '<div class="pendingCard">',
          '<div class="pendingTitle">Diff preview</div>',
          renderCompactDiff(message.diffPreview),
          "</div>",
        ].join("");
      }

      function renderLiveCard(message) {
        if (!message.liveStatus) {
          return "";
        }
        const chips = Array.isArray(message.liveTrace) && message.liveTrace.length
          ? '<div class="liveTrace">' + message.liveTrace.map((item) => '<div class="liveChip">' + escapeHtml(item) + '</div>').join("") + "</div>"
          : "";
        return [
          '<div class="liveCard">',
          '<div class="liveRow"><div class="liveDot"></div><div class="liveTitle">' + escapeHtml(message.liveStatus) + "</div></div>",
          chips,
          "</div>",
        ].join("");
      }

      function renderMessages(messages) {
        if (!messages.length) {
          return '<article class="msg assistant"><div class="role">DorkPipe</div><div class="body"><p>Ask about this workspace. DorkPipe will use the local context bundle when available, keep chat history per workspace, and route obvious safe actions locally first.</p></div></article>';
        }
        return messages.map((message) => {
          const role = message.role === "assistant" ? "DorkPipe" : "You";
          return '<article class="msg ' + message.role + '"><div class="role">' + role + '</div><div class="body">' + (message.html || "") + renderLiveCard(message) + renderDiffPreview(message) + renderPendingAction(message) + '</div></article>';
        }).join("");
      }

      function renderTrace(items) {
        if (!items || !items.length) {
          return "";
        }
        return items.map((item) => '<div class="traceItem">' + escapeHtml(item) + '</div>').join("");
      }

      function renderSessions(nextState) {
        const sessions = nextState.sessionList || [];
        sessionSelect.innerHTML = sessions.map((session) => {
          const selected = session.id === nextState.activeSessionId ? " selected" : "";
          return '<option value="' + escapeHtml(session.id) + '"' + selected + '>' + escapeHtml(session.title) + '</option>';
        }).join("");
      }

      function saveViewState(extra) {
        viewState = { ...viewState, ...extra };
        vscode.setState(viewState);
      }

      function submitPrompt() {
        const text = prompt.value.trim();
        postDiag("submit-attempt", { chars: text.length, mode: modeSelect.value, profile: modelProfileSelect.value });
        if (!text) return;
        prompt.value = "";
        saveViewState({ draft: "", pinnedToBottom: true, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
        vscode.postMessage({ type: "ask", text, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
      }

      function render(nextState) {
        try {
          currentState = nextState || {};
          const previousBottomOffset = transcript.scrollHeight - transcript.scrollTop;
          const stickToBottom = previousBottomOffset - transcript.clientHeight <= 24 || !!viewState.pinnedToBottom;
          transcript.innerHTML = renderMessages(currentState.messages || []);
          trace.innerHTML = renderTrace(currentState.trace || []);
          status.textContent = currentState.status || "";
          renderSessions(currentState);
          modeSelect.value = currentState.composerMode || viewState.mode || "ask";
          autoApplyEdits.checked = !!currentState.autoApplyEdits;
          modelProfileSelect.value = currentState.modelProfile || "balanced";
          send.disabled = !!currentState.isBusy;
          clearBtn.disabled = !!currentState.isBusy;
          newChatBtn.disabled = !!currentState.isBusy;
          newFromComposer.disabled = !!currentState.isBusy;
          modeSelect.disabled = !!currentState.isBusy;
          autoApplyEdits.disabled = !!currentState.isBusy;
          modelProfileSelect.disabled = !!currentState.isBusy;
          if (stickToBottom) {
            transcript.scrollTop = transcript.scrollHeight;
          } else {
            transcript.scrollTop = Math.max(0, transcript.scrollHeight - previousBottomOffset);
          }
          saveViewState({ pinnedToBottom: stickToBottom });
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          transcript.innerHTML = '<article class="msg assistant"><div class="role">DorkPipe</div><div class="body"><p>UI render failed.</p><p><code>' + escapeHtml(message) + '</code></p></div></article>';
          status.textContent = "DorkPipe UI hit a render error";
        }
      }

      try {
        transcript.addEventListener("scroll", () => {
          const bottomGap = transcript.scrollHeight - transcript.scrollTop - transcript.clientHeight;
          saveViewState({ pinnedToBottom: bottomGap <= 24 });
        });

        prompt.value = viewState.draft || "";
        modeSelect.value = viewState.mode || currentState.composerMode || "ask";
        modelProfileSelect.value = viewState.modelProfile || currentState.modelProfile || "balanced";
        prompt.addEventListener("input", () => {
          saveViewState({ draft: prompt.value });
        });
        modeSelect.addEventListener("change", () => {
          saveViewState({ mode: modeSelect.value });
          vscode.postMessage({ type: "setComposerMode", mode: modeSelect.value });
        });
        autoApplyEdits.addEventListener("change", () => {
          vscode.postMessage({ type: "setAutoApplyEdits", value: autoApplyEdits.checked });
        });
        modelProfileSelect.addEventListener("change", () => {
          vscode.postMessage({ type: "setModelProfile", value: modelProfileSelect.value });
        });
        prompt.addEventListener("keydown", (event) => {
          if (event.key !== "Enter") {
            return;
          }
          if (event.isComposing) {
            return;
          }
          if (event.altKey) {
            return;
          }
          if (event.shiftKey || event.ctrlKey || event.metaKey) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          submitPrompt();
        });

        sessionSelect.addEventListener("change", () => {
          vscode.postMessage({ type: "switchSession", sessionId: sessionSelect.value });
        });

        clearBtn.addEventListener("click", () => {
          vscode.postMessage({ type: "clearSession" });
        });

        function startNewChat() {
          vscode.postMessage({ type: "newSession", seed: prompt.value.trim() });
        }

        newChatBtn.addEventListener("click", startNewChat);
        newFromComposer.addEventListener("click", startNewChat);

        form.addEventListener("submit", (event) => {
          event.preventDefault();
          submitPrompt();
        });

        transcript.addEventListener("click", (event) => {
          const target = event.target instanceof HTMLElement ? event.target.closest("[data-pending-action]") : null;
          if (!target) {
            return;
          }
          vscode.postMessage({
            type: "resolvePendingAction",
            messageId: target.getAttribute("data-message-id"),
            decision: target.getAttribute("data-pending-action"),
          });
        });

        window.addEventListener("message", (event) => {
          const msg = event.data || {};
          postDiag("host-message", { type: msg.type || "" });
          if (msg.type === "state" && msg.state) {
            render(msg.state);
          }
          if (msg.type === "done") {
            prompt.value = "";
            saveViewState({ draft: "", pinnedToBottom: true });
            send.disabled = false;
            prompt.focus();
          }
        });

        postDiag("listeners-attached");
        render(currentState);
        postDiag("initial-render-complete", {
          messages: Array.isArray(currentState?.messages) ? currentState.messages.length : 0,
        });
        vscode.postMessage({ type: "webviewReady" });
        postDiag("ready-sent");
        prompt.focus();
        postDiag("focus-complete");
      } catch (error) {
        reportClientError("boot", error);
      }
    </script>
  </body>
</html>`;
}
function getWebviewAssetUri(webview, extensionUri, ...segments) {
    return webview.asWebviewUri(vscode.Uri.joinPath(extensionUri, ...segments));
}
function renderChatHtmlExternal(webview, extensionUri, state) {
    const nonce = String(Date.now());
    const initialStateJson = serializeForInlineScript({
        ...(state || {}),
        shellVersion: CHAT_WEBVIEW_SHELL_VERSION,
    });
    const styleUri = getWebviewAssetUri(webview, extensionUri, "webview", "chat.css");
    const scriptUri = getWebviewAssetUri(webview, extensionUri, "webview", "chat.js");
    return `<!doctype html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta
      http-equiv="Content-Security-Policy"
      content="default-src 'none'; img-src ${webview.cspSource} https: data:; style-src ${webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}' ${webview.cspSource};"
    />
    <link rel="stylesheet" href="${styleUri}" />
  </head>
  <body>
    <div class="wrap">
      <header class="header">
        <div class="titleRow">
          <div class="titleBlock">
            <div class="titleIcon">←</div>
            <div class="titleStack">
              <h1 class="title">Workspace Chat</h1>
              <div class="headerMeta" id="headerMeta"></div>
            </div>
          </div>
          <div class="headerSpacer"></div>
          <div class="headerActions">
            <select id="sessionSelect" class="headerSelect" aria-label="Chat session"></select>
            <button class="btn ghost iconBtn" id="settingsBtn" type="button" title="Reasoning templates and models">⚙</button>
            <button class="btn ghost iconBtn" id="clearBtn" type="button" title="Clear chat">↺</button>
            <button class="btn ghost iconBtn" id="newChatBtn" type="button" title="New chat">✎</button>
          </div>
        </div>
        <div class="trace" id="trace"></div>
      </header>
      <div class="bootSentinel" id="bootSentinel">DorkPipe UI is waiting for client-side boot. If this stays visible, the webview script did not start.</div>
      <main class="transcript" id="transcript"></main>
      <section class="studioSurface hidden" id="studioSurface" aria-hidden="true">
        <div class="studioHeader">
          <div>
            <div class="settingsEyebrow">DockPipe Studio</div>
            <h2 id="studioTitle">Template Designer</h2>
            <div class="canvasHint" id="studioMeta">Inspect and shape the active DockPipe reasoning surface.</div>
          </div>
          <div class="toolbarRow">
            <button class="btn ghost" id="studioBackBtn" type="button">Back to chat</button>
          </div>
        </div>
        <div class="studioScroll">
          <section class="settingsSection hidden" id="settingsStudio">
            <div class="sectionHead">
              <div>
                <h3>Workspace Settings</h3>
                <p>Review the active reasoning surface here, then jump into the dedicated template or model editors.</p>
              </div>
            </div>
            <div class="settingsSummaryGrid">
              <article class="summaryCard">
                <div class="summaryLabel">Active template</div>
                <div class="summaryValue" id="activeTemplateSummary">DockPipe Default</div>
              </article>
              <article class="summaryCard">
                <div class="summaryLabel">Templates</div>
                <div class="summaryValue" id="templateCountSummary">0 available</div>
              </article>
              <article class="summaryCard">
                <div class="summaryLabel">Models</div>
                <div class="summaryValue" id="modelCountSummary">0 registered</div>
              </article>
            </div>
            <div class="settingsSection" style="margin-top:12px;">
              <div class="sectionHead">
                <div>
                  <h3>Templates</h3>
                  <p>Select the active template or jump straight into the designer for edits.</p>
                </div>
                <div class="toolbarRow compactRow">
                  <button class="btn" id="newTemplateBtn" type="button">Create template</button>
                  <button class="btn ghost" id="openTemplateDesignerBtn" type="button">Open designer</button>
                </div>
              </div>
              <select id="templateSelect" class="hidden" aria-label="Active template"></select>
              <div class="templateManagerList" id="templateManagerList"></div>
            </div>
            <div class="settingsSection" style="margin-top:12px;">
              <div class="sectionHead">
                <div>
                  <h3>Models</h3>
                  <p>Open the full model manager in the main area when you need to register, review, or delete entries.</p>
                </div>
              </div>
              <div class="modelStoreSummary" id="modelStoreSummary"></div>
              <div class="toolbarRow">
                <button class="btn" id="openModelManagerBtn" type="button">Open model manager</button>
              </div>
            </div>
          </section>
          <section class="settingsSection" id="templateStudio">
            <div class="sectionHead">
              <div>
                <h3>Visual Designer</h3>
                <p>Edit the active template here. Build the flow visually, then save or duplicate it once the shape feels right.</p>
              </div>
              <div class="toolbarRow compactRow">
                <button class="btn ghost" id="copyTemplateBtn" type="button">Duplicate</button>
                <button class="btn ghost danger" id="deleteTemplateBtn" type="button">Delete</button>
                <button class="btn" id="saveTemplateBtn" type="button">Save template</button>
              </div>
            </div>
            <div class="designerLayout">
              <div class="designerPalette">
                <div class="paletteLabel">Primitives</div>
                <button class="primitiveTile" type="button" draggable="true" data-primitive-type="dockpipe">DockPipe</button>
                <button class="primitiveTile" type="button" draggable="true" data-primitive-type="model">Model</button>
                <button class="primitiveTile" type="button" draggable="true" data-primitive-type="loop">Loop Control</button>
              </div>
              <div class="designerCanvasWrap">
                <div class="canvasToolbar">
                  <span class="canvasLabel">Canvas</span>
                  <span class="canvasHint">Drop a primitive onto the root canvas or into a loop body.</span>
                </div>
                <div class="designerCanvas" id="designerCanvas"></div>
              </div>
              <div class="designerInspector">
                <div class="inspectorHeader">
                  <div class="paletteLabel">Inspector</div>
                  <div class="canvasHint" id="inspectorTarget">Template</div>
                </div>
                <div class="inspectorScroll">
                  <div class="inspectorGroup">
                    <label class="field">
                      <span>Template name</span>
                      <input id="templateNameInput" type="text" />
                    </label>
                    <label class="field">
                      <span>Description</span>
                      <textarea id="templateDescriptionInput" rows="3"></textarea>
                    </label>
                  </div>
                  <div class="inspectorGroup">
                    <h4>Global modifiers</h4>
                    <label class="field">
                      <span>Safety rules</span>
                      <textarea id="templateSafetyRulesInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Output requirements</span>
                      <textarea id="templateOutputRequirementsInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Confidence threshold</span>
                      <input id="templateConfidenceInput" type="number" min="0" max="1" step="0.01" />
                    </label>
                    <label class="field">
                      <span>Execution constraints</span>
                      <textarea id="templateExecutionConstraintsInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Routing preference</span>
                      <input id="templateRoutingPreferenceInput" type="text" />
                    </label>
                  </div>
                  <div class="inspectorGroup">
                    <h4>Decision and guidance</h4>
                    <label class="field">
                      <span>Template orchestration guidance</span>
                      <textarea id="templateOrchestrationGuidanceInput" rows="3"></textarea>
                    </label>
                    <label class="field">
                      <span>Template model guidance</span>
                      <textarea id="templateModelGuidanceInput" rows="3"></textarea>
                    </label>
                  </div>
                  <div class="inspectorGroup">
                    <h4>Selected node</h4>
                    <label class="field">
                      <span>Label</span>
                      <input id="nodeLabelInput" type="text" />
                    </label>
                    <label class="field">
                      <span>Notes</span>
                      <textarea id="nodeNotesInput" rows="2"></textarea>
                    </label>
                    <label class="field">
                      <span>Decision metadata</span>
                      <textarea id="nodeDecisionInput" rows="2"></textarea>
                    </label>
                    <label class="field">
                      <span>Node orchestration guidance</span>
                      <textarea id="nodeOrchestrationInput" rows="2"></textarea>
                    </label>
                    <label class="field">
                      <span>Node model guidance</span>
                      <textarea id="nodeModelInput" rows="2"></textarea>
                    </label>
                    <label class="field" id="nodeModelField">
                      <span>Model</span>
                      <select id="nodeModelSelect"></select>
                    </label>
                    <label class="field" id="nodeTaskField">
                      <span>Task / phase</span>
                      <input id="nodeTaskInput" type="text" />
                    </label>
                    <label class="field" id="nodeOutputField">
                      <span>Output mode</span>
                      <input id="nodeOutputInput" type="text" />
                    </label>
                    <label class="field" id="nodeLoopIterationsField">
                      <span>Loop iterations</span>
                      <input id="nodeLoopIterationsInput" type="number" min="1" max="8" step="1" />
                    </label>
                    <div class="toolbarRow">
                      <button class="btn ghost danger" id="removeNodeBtn" type="button">Remove node</button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>
          <section class="settingsSection hidden" id="modelStudio">
            <div class="sectionHead">
              <div>
                <h3>Model Store</h3>
                <p>Reasoning templates reference registered models from this local store rather than raw strings.</p>
              </div>
            </div>
            <div class="modelStoreSummary" id="modelStoreSummaryStudio"></div>
            <div class="modelStoreList" id="modelStoreList"></div>
            <div class="modelEditor">
              <div class="paletteLabel">Register custom model</div>
              <div class="formGrid">
                <label class="field"><span>Model id</span><input id="modelEntryIdInput" type="text" /></label>
                <label class="field"><span>Label</span><input id="modelEntryLabelInput" type="text" /></label>
                <label class="field"><span>Provider</span><input id="modelEntryProviderInput" type="text" value="ollama" /></label>
                <label class="field"><span>Model name</span><input id="modelEntryModelInput" type="text" /></label>
                <label class="field"><span>Capabilities</span><input id="modelEntryCapabilitiesInput" type="text" placeholder="chat, edit, reasoning" /></label>
                <label class="field"><span>Context window</span><input id="modelEntryContextWindowInput" type="number" min="1024" step="1024" value="8192" /></label>
                <label class="field fieldWide"><span>Notes</span><textarea id="modelEntryNotesInput" rows="2"></textarea></label>
              </div>
              <div class="toolbarRow">
                <button class="btn" id="saveModelEntryBtn" type="button">Save model</button>
              </div>
            </div>
          </section>
          <section class="settingsSection hidden" id="runStudio">
            <div class="sectionHead">
              <div>
                <h3>Run Inspector</h3>
                <p>Inspect the structured edit plan, execution trace, and validation results for a prepared or applied run.</p>
              </div>
            </div>
            <div class="runInspectorBody" id="runInspectorBody">
              <div class="emptyBlock">Pick an assistant run from the chat transcript to inspect it here.</div>
            </div>
          </section>
        </div>
      </section>
      <div class="composerWrap">
        <div class="status" id="status"></div>
        <form class="composer" id="composer">
          <textarea id="prompt" placeholder="Ask DorkPipe about this repo... or use /help for local commands"></textarea>
          <div class="actions">
            <div class="controlsMeta">
              <div class="toolRow">
                <label class="toggleWrap"><input id="autoApplyEdits" type="checkbox" /> Auto</label>
                <select id="modelProfileSelect" aria-label="Model profile">
                  <option value="fast">Fast</option>
                  <option value="balanced">Balanced</option>
                  <option value="deep">Deep</option>
                  <option value="max">Max</option>
                </select>
                <select id="modeSelect" aria-label="Request mode">
                  <option value="ask">Ask</option>
                  <option value="agent">Agent</option>
                  <option value="plan">Plan</option>
                </select>
              </div>
              <div class="hint">Local-first routing with safe primitives, bounded tools, and streamed model steps.</div>
            </div>
            <div class="sendWrap">
              <button class="btn ghost" id="newFromComposer" type="button">New</button>
              <button class="btn" id="send" type="submit">Send</button>
            </div>
          </div>
        </form>
      </div>
    </div>
    <script type="application/json" id="pipeon-initial-state">${initialStateJson}</script>
    <script nonce="${nonce}" src="${scriptUri}"></script>
  </body>
</html>`;
}
function renderCanaryHtml(webview, extensionUri) {
    const nonce = String(Date.now());
    const scriptUri = getWebviewAssetUri(webview, extensionUri, "webview", "canary.js");
    return `<!doctype html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta
      http-equiv="Content-Security-Policy"
      content="default-src 'none'; style-src ${webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}' ${webview.cspSource};"
    />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <style>
      body {
        margin: 0;
        padding: 16px;
        font-family: var(--vscode-font-family);
        color: var(--vscode-editor-foreground);
        background: var(--vscode-editor-background);
      }
      .card {
        border: 1px solid var(--vscode-panel-border);
        border-radius: 14px;
        padding: 16px;
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 94%, transparent);
      }
      .status {
        margin: 0 0 12px;
        font-size: 13px;
      }
      button {
        border: 0;
        border-radius: 10px;
        padding: 10px 14px;
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        font: inherit;
        font-weight: 600;
        cursor: pointer;
      }
    </style>
  </head>
  <body>
    <div class="card">
      <p id="status" class="status">canary: shell</p>
      <button id="ping" type="button">Ping host</button>
    </div>
    <script nonce="${nonce}" src="${scriptUri}"></script>
  </body>
</html>`;
}
function renderWelcomeHtml(webview, extensionUri) {
    const nonce = String(Date.now());
    const iconUri = webview.asWebviewUri(vscode.Uri.joinPath(extensionUri, "images", "icon.png"));
    return `<!doctype html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${webview.cspSource} https: data:; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <style>
      body {
        margin: 0;
        font-family: var(--vscode-font-family);
        color: var(--vscode-editor-foreground);
        background:
          radial-gradient(circle at top left, color-mix(in srgb, var(--vscode-button-background) 22%, transparent), transparent 45%),
          linear-gradient(180deg, color-mix(in srgb, var(--vscode-sideBar-background) 94%, transparent), var(--vscode-editor-background));
      }
      .wrap {
        min-height: 100vh;
        display: grid;
        grid-template-columns: minmax(320px, 560px) 1fr;
        gap: 32px;
        align-items: center;
        padding: 48px;
      }
      .copy {
        max-width: 520px;
      }
      .eyebrow {
        text-transform: uppercase;
        letter-spacing: 0.14em;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
        margin-bottom: 12px;
      }
      h1 {
        margin: 0 0 16px;
        font-size: 42px;
        line-height: 1.05;
      }
      p {
        margin: 0 0 16px;
        font-size: 17px;
        line-height: 1.6;
        color: var(--vscode-descriptionForeground);
      }
      .actions {
        display: flex;
        gap: 12px;
        flex-wrap: wrap;
        margin-top: 20px;
      }
      button {
        border: 0;
        border-radius: 12px;
        padding: 12px 18px;
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        font: inherit;
        font-weight: 600;
        cursor: pointer;
      }
      button.secondary {
        background: color-mix(in srgb, var(--vscode-button-background) 15%, transparent);
        color: var(--vscode-editor-foreground);
        border: 1px solid var(--vscode-panel-border);
      }
      .hero {
        display: flex;
        justify-content: center;
      }
      .card {
        width: min(100%, 560px);
        aspect-ratio: 1.2 / 1;
        border-radius: 28px;
        border: 1px solid color-mix(in srgb, var(--vscode-button-background) 22%, var(--vscode-panel-border));
        background:
          radial-gradient(circle at 30% 25%, color-mix(in srgb, var(--vscode-button-background) 24%, transparent), transparent 30%),
          linear-gradient(180deg, color-mix(in srgb, var(--vscode-editorWidget-background) 96%, transparent), color-mix(in srgb, var(--vscode-editorWidget-background) 88%, transparent));
        box-shadow: 0 24px 60px rgba(0,0,0,0.22);
        display: grid;
        place-items: center;
      }
      .card img {
        width: min(62%, 320px);
        height: auto;
        filter: drop-shadow(0 18px 30px rgba(0,0,0,0.35));
      }
      @media (max-width: 960px) {
        .wrap {
          grid-template-columns: 1fr;
          padding: 28px;
        }
        h1 { font-size: 34px; }
      }
    </style>
  </head>
  <body>
    <div class="wrap">
      <section class="copy">
        <div class="eyebrow">Pipeon</div>
        <h1>Get Started with Pipeon</h1>
        <p>Local-first coding with a proper app shell, DorkPipe in-editor help, DockPipe workflows, and your repo context already wired in.</p>
        <p>Open chat, inspect the local bundle, or jump straight into the workspace without the stock web welcome getting in the way.</p>
        <div class="actions">
          <button data-command="chat">Open DorkPipe Chat</button>
          <button class="secondary" data-command="context">Open Context Bundle</button>
          <button class="secondary" data-command="docs">Open Docs</button>
        </div>
      </section>
      <section class="hero">
        <div class="card">
          <img src="${iconUri}" alt="Pipeon" />
        </div>
      </section>
    </div>
    <script nonce="${nonce}">
      const vscode = acquireVsCodeApi();
      for (const button of document.querySelectorAll('button[data-command]')) {
        button.addEventListener('click', () => {
          vscode.postMessage({ type: 'command', command: button.dataset.command });
        });
      }
    </script>
  </body>
</html>`;
}
function openPipeonWelcome(context) {
    const panel = vscode.window.createWebviewPanel(WELCOME_PANEL_ID, "Pipeon", vscode.ViewColumn.Active, {
        enableScripts: true,
        retainContextWhenHidden: true,
    });
    panel.webview.html = renderWelcomeHtml(panel.webview, context.extensionUri);
    panel.webview.onDidReceiveMessage(async (msg) => {
        switch (msg?.command) {
            case "chat":
                await vscode.commands.executeCommand("pipeon.openChat");
                break;
            case "context":
                await vscode.commands.executeCommand("pipeon.openContextBundle");
                break;
            case "docs":
                await vscode.commands.executeCommand("pipeon.showReadme");
                break;
            default:
                break;
        }
    });
    return panel;
}
function looksLikeStockWelcomeTab(tab) {
    const label = String(tab?.label || "").toLowerCase();
    return label.includes("welcome") || label.includes("get started");
}
async function closeStockWelcomeTabs() {
    const groups = vscode.window.tabGroups?.all || [];
    const tabsToClose = [];
    for (const group of groups) {
        for (const tab of group.tabs) {
            if (looksLikeStockWelcomeTab(tab)) {
                tabsToClose.push(tab);
            }
        }
    }
    if (tabsToClose.length === 0) {
        return false;
    }
    try {
        await vscode.window.tabGroups.close(tabsToClose);
        return true;
    }
    catch {
        try {
            await vscode.commands.executeCommand("workbench.action.closeAllEditors");
            return true;
        }
        catch {
            return false;
        }
    }
}
async function replaceStockWelcomeWithPipeon(context) {
    const groups = vscode.window.tabGroups?.all || [];
    let foundWelcome = false;
    let foundNonWelcome = false;
    for (const group of groups) {
        for (const tab of group.tabs) {
            if (looksLikeStockWelcomeTab(tab)) {
                foundWelcome = true;
            }
            else {
                foundNonWelcome = true;
            }
        }
    }
    if (!foundWelcome || foundNonWelcome) {
        return;
    }
    await closeStockWelcomeTabs();
    openPipeonWelcome(context);
}
async function redirectStockWelcomeToPipeon(context) {
    const groups = vscode.window.tabGroups?.all || [];
    let foundWelcome = false;
    for (const group of groups) {
        for (const tab of group.tabs) {
            if (looksLikeStockWelcomeTab(tab)) {
                foundWelcome = true;
                break;
            }
        }
        if (foundWelcome) {
            break;
        }
    }
    if (!foundWelcome) {
        return;
    }
    const closed = await closeStockWelcomeTabs();
    if (closed) {
        openPipeonWelcome(context);
    }
}
async function revealDorkpipePanel() {
    await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
}
function configureChatWebview(webview, extensionUri) {
    webview.options = {
        enableScripts: true,
        localResourceRoots: [
            vscode.Uri.joinPath(extensionUri, "webview"),
            vscode.Uri.joinPath(extensionUri, "images"),
        ],
    };
}
async function resetPanelToBottomOnce(context) {
    if (context.globalState.get(PANEL_BOTTOM_MIGRATION_KEY)) {
        return;
    }
    await context.globalState.update(PANEL_BOTTOM_MIGRATION_KEY, true);
    try {
        await vscode.commands.executeCommand("workbench.action.positionPanelBottom");
    }
    catch {
        // Best-effort cleanup for the old broken layout migration.
    }
}
class PipeonChatViewProvider {
    context;
    channel;
    extensionVersion;
    view;
    surfaces;
    detachedPanel;
    detachedSurfaceId;
    chatStore;
    state;
    constructor(context, channel, extensionVersion) {
        this.context = context;
        this.channel = channel;
        this.extensionVersion = extensionVersion || "unknown";
        this.view = null;
        this.surfaces = new Map();
        this.detachedPanel = null;
        this.detachedSurfaceId = "";
        logChannelInfo(this.channel, "Pipeon chat provider constructing");
        try {
            this.chatStore = normalizeStoredChatState(this.context.workspaceState.get(CHAT_STATE_KEY));
            logChannelInfo(this.channel, "Chat history restored", {
                sessions: Array.isArray(this.chatStore?.sessions) ? this.chatStore.sessions.length : 0,
                activeSessionId: this.chatStore?.activeSessionId || "",
            });
        }
        catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.channel.error(`Failed to restore chat history: ${message}`);
            this.chatStore = createInitialChatState();
        }
        this.state = {
            sessionList: [],
            activeSessionId: this.chatStore.activeSessionId,
            composerMode: this.chatStore.composerMode || "ask",
            autoApplyEdits: !!this.chatStore.autoApplyEdits,
            modelProfile: normalizeModelProfile(this.chatStore.modelProfile),
            activeTemplateId: this.chatStore.activeTemplateId || BUILTIN_TEMPLATE_ID,
            activeTemplate: null,
            reasoningTemplates: [],
            modelStore: { entries: [], updatedAt: nowIso() },
            messages: [],
            trace: [],
            status: "Waiting for workspace...",
            isBusy: false,
            shellVersion: CHAT_WEBVIEW_SHELL_VERSION,
            extensionVersion: this.extensionVersion,
        };
        this.syncViewState();
    }
    postToSurfaces(message) {
        for (const surface of this.surfaces.values()) {
            try {
                surface.webview.postMessage(message);
            }
            catch {
                // Best-effort broadcast.
            }
        }
    }
    postToSurface(surfaceId, message) {
        const surface = this.surfaces.get(surfaceId);
        if (!surface) {
            return;
        }
        try {
            surface.webview.postMessage(message);
        }
        catch {
            // Best-effort targeted post.
        }
    }
    get activeSession() {
        this.chatStore = ensureValidChatStore(this.chatStore);
        return this.chatStore.sessions.find((session) => session.id === this.chatStore.activeSessionId) || this.chatStore.sessions[0];
    }
    async persistChatStore() {
        const compact = compactChatStoreForPersistence(this.chatStore);
        try {
            await this.context.workspaceState.update(CHAT_STATE_KEY, compact);
        }
        catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.channel.error(`Failed to persist chat history, retrying with a smaller snapshot: ${message}`);
            const fallback = {
                ...compact,
                sessions: compact.sessions.slice(0, Math.min(6, MAX_SAVED_SESSIONS)).map((session) => ({
                    ...session,
                    messages: Array.isArray(session.messages) ? session.messages.slice(-Math.min(24, MAX_HISTORY_MESSAGES)) : [],
                })),
            };
            try {
                await this.context.workspaceState.update(CHAT_STATE_KEY, fallback);
            }
            catch (fallbackError) {
                const fallbackMessage = fallbackError instanceof Error ? fallbackError.message : String(fallbackError);
                this.channel.error(`Failed to persist compact chat history: ${fallbackMessage}`);
            }
        }
    }
    syncViewState() {
        this.chatStore = ensureValidChatStore(this.chatStore);
        const session = this.activeSession || createSession();
        const activeTemplate = templateById(this.chatStore.reasoningTemplates, this.chatStore.activeTemplateId);
        this.state.activeSessionId = session.id;
        this.state.composerMode = this.chatStore.composerMode || "ask";
        this.state.autoApplyEdits = !!this.chatStore.autoApplyEdits;
        this.state.modelProfile = normalizeModelProfile(this.chatStore.modelProfile);
        this.state.activeTemplateId = activeTemplate?.id || BUILTIN_TEMPLATE_ID;
        this.state.activeTemplate = activeTemplate ? summarizeTemplate(activeTemplate) : null;
        this.state.reasoningTemplates = normalizeReasoningTemplates(this.chatStore.reasoningTemplates, this.chatStore.modelStore).map((template) => deepClone(template));
        this.state.modelStore = deepClone(normalizeModelStore(this.chatStore.modelStore));
        this.state.sessionList = sortSessionsByUpdate(this.chatStore.sessions).map((item) => ({
            id: item.id,
            title: item.title || "New chat",
        }));
        this.state.messages = session.messages.map((message) => ({
            ...message,
            html: safeRenderMessageBody(message, this.channel),
        }));
    }
    async focus() {
        await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
    }
    refresh() {
        this.syncViewState();
        for (const [surfaceId, surface] of this.surfaces.entries()) {
            try {
                if (!surface.rendered) {
                    logChannelInfo(this.channel, "Rendering chat webview shell", {
                        surfaceId,
                        activeSessionId: this.state.activeSessionId,
                        messages: Array.isArray(this.state.messages) ? this.state.messages.length : 0,
                        shellVersion: CHAT_WEBVIEW_SHELL_VERSION,
                    });
                    surface.webview.html = renderChatHtmlExternal(surface.webview, this.context.extensionUri, this.state);
                    surface.rendered = true;
                    surface.shellVersion = "";
                }
                else {
                    logChannelInfo(this.channel, "Posting state to chat webview", {
                        surfaceId,
                        activeSessionId: this.state.activeSessionId,
                        messages: Array.isArray(this.state.messages) ? this.state.messages.length : 0,
                        isBusy: !!this.state.isBusy,
                        shellVersion: surface.shellVersion || "unknown",
                    });
                    surface.webview.postMessage({ type: "state", state: this.state });
                }
            }
            catch (error) {
                const message = error instanceof Error ? error.message : String(error);
                this.channel.error(`Webview refresh failed (${surfaceId}): ${message}`);
                try {
                    logChannelInfo(this.channel, "Attempting full webview reload after refresh failure", { surfaceId });
                    surface.webview.html = renderChatHtmlExternal(surface.webview, this.context.extensionUri, this.state);
                    surface.rendered = true;
                    surface.shellVersion = "";
                }
                catch (reloadError) {
                    const reloadMessage = reloadError instanceof Error ? reloadError.message : String(reloadError);
                    this.channel.error(`Webview reload failed (${surfaceId}): ${reloadMessage}`);
                    surface.rendered = false;
                }
            }
        }
    }
    async saveAndRefresh() {
        await this.persistChatStore();
        this.refresh();
    }
    async newSession(seedText = "") {
        const session = createSession(seedText);
        this.chatStore.sessions = [session, ...this.chatStore.sessions].slice(0, MAX_SAVED_SESSIONS);
        this.chatStore.activeSessionId = session.id;
        this.state.trace = [];
        this.state.status = "Started a new chat.";
        await this.saveAndRefresh();
    }
    async clearActiveSession() {
        const session = this.activeSession;
        session.messages = [];
        session.updatedAt = nowIso();
        session.title = "New chat";
        this.state.trace = [];
        this.state.status = "Cleared current chat.";
        await this.saveAndRefresh();
    }
    async switchSession(sessionId) {
        const found = this.chatStore.sessions.find((session) => session.id === sessionId);
        if (!found) {
            return;
        }
        this.chatStore.activeSessionId = found.id;
        this.state.trace = [];
        this.state.status = `Viewing chat: ${found.title}`;
        await this.saveAndRefresh();
    }
    async setComposerMode(mode) {
        const nextMode = ["ask", "agent", "plan"].includes(String(mode || "").toLowerCase())
            ? String(mode).toLowerCase()
            : "ask";
        this.chatStore.composerMode = nextMode;
        this.state.composerMode = nextMode;
        await this.saveAndRefresh();
    }
    async setAutoApplyEdits(value) {
        this.chatStore.autoApplyEdits = !!value;
        this.state.autoApplyEdits = !!value;
        await this.saveAndRefresh();
    }
    async setModelProfile(value) {
        this.chatStore.modelProfile = normalizeModelProfile(value);
        this.state.modelProfile = normalizeModelProfile(value);
        await this.saveAndRefresh();
    }
    async resetWebviewShell(reason = "Manual reset") {
        for (const surface of this.surfaces.values()) {
            surface.rendered = false;
            surface.shellVersion = "";
        }
        this.state.trace = [];
        this.state.isBusy = false;
        this.state.status = `Rebuilding DorkPipe chat shell (${reason})`;
        this.refresh();
    }
    sidebarShellLooksStale() {
        const surface = this.surfaces.get("sidebar");
        if (!surface) {
            return false;
        }
        return !!surface.shellVersion && surface.shellVersion !== CHAT_WEBVIEW_SHELL_VERSION;
    }
    async setActiveTemplate(templateId) {
        const next = templateById(this.chatStore.reasoningTemplates, templateId);
        if (!next) {
            return;
        }
        this.chatStore.activeTemplateId = next.id;
        this.state.status = `Using reasoning template: ${next.name}`;
        await this.saveAndRefresh();
    }
    async createTemplate(seedTemplateId) {
        const clone = seedTemplateId === "__blank__"
            ? normalizeReasoningTemplate(createBlankReasoningTemplate(this.chatStore.modelStore, "Blank Template"), this.chatStore.modelStore)
            : normalizeReasoningTemplate({
                ...deepClone(templateById(this.chatStore.reasoningTemplates, seedTemplateId || this.chatStore.activeTemplateId)
                    || createDefaultReasoningTemplate(this.chatStore.modelStore)),
                id: makeId("template"),
                name: `${(templateById(this.chatStore.reasoningTemplates, seedTemplateId || this.chatStore.activeTemplateId)?.name || "Template")} Copy`,
                description: templateById(this.chatStore.reasoningTemplates, seedTemplateId || this.chatStore.activeTemplateId)?.description || "Custom reasoning template.",
                builtIn: false,
                locked: false,
                createdAt: nowIso(),
                updatedAt: nowIso(),
            }, this.chatStore.modelStore);
        this.chatStore.reasoningTemplates = [templateById(this.chatStore.reasoningTemplates, BUILTIN_TEMPLATE_ID), ...this.chatStore.reasoningTemplates.filter((item) => item.id !== BUILTIN_TEMPLATE_ID), clone]
            .filter(Boolean)
            .slice(0, MAX_REASONING_TEMPLATES);
        this.chatStore.activeTemplateId = clone.id;
        this.state.status = `Created reasoning template: ${clone.name}`;
        await this.saveAndRefresh();
    }
    async saveTemplate(template) {
        const existing = templateById(this.chatStore.reasoningTemplates, template?.id);
        if (existing?.locked) {
            return;
        }
        const normalized = normalizeReasoningTemplate(template, this.chatStore.modelStore, existing || createBlankReasoningTemplate(this.chatStore.modelStore));
        const templates = this.chatStore.reasoningTemplates.filter((item) => item.id !== normalized.id);
        this.chatStore.reasoningTemplates = normalizeReasoningTemplates([...templates, normalized], this.chatStore.modelStore);
        this.chatStore.activeTemplateId = normalized.id;
        this.state.status = `Saved reasoning template: ${normalized.name}`;
        await this.saveAndRefresh();
    }
    async deleteTemplate(templateId) {
        const existing = templateById(this.chatStore.reasoningTemplates, templateId);
        if (!existing || existing.locked) {
            return;
        }
        this.chatStore.reasoningTemplates = normalizeReasoningTemplates(this.chatStore.reasoningTemplates.filter((item) => item.id !== existing.id), this.chatStore.modelStore);
        this.chatStore.activeTemplateId = BUILTIN_TEMPLATE_ID;
        this.state.status = `Deleted reasoning template: ${existing.name}`;
        await this.saveAndRefresh();
    }
    async upsertModelEntry(entry) {
        const existing = normalizeModelStore(this.chatStore.modelStore).entries.find((item) => item.id === entry?.id);
        if (existing?.locked) {
            return;
        }
        const normalized = normalizeModelEntry({
            ...entry,
            builtIn: false,
            locked: false,
            installState: "custom",
        });
        const entries = normalizeModelStore(this.chatStore.modelStore).entries.filter((item) => item.id !== normalized.id);
        this.chatStore.modelStore = normalizeModelStore({ entries: [...entries, normalized], updatedAt: nowIso() });
        this.chatStore.reasoningTemplates = normalizeReasoningTemplates(this.chatStore.reasoningTemplates, this.chatStore.modelStore);
        this.state.status = `Saved model entry: ${normalized.label}`;
        await this.saveAndRefresh();
    }
    async deleteModelEntry(modelId) {
        const existing = normalizeModelStore(this.chatStore.modelStore).entries.find((item) => item.id === modelId);
        if (!existing || existing.locked) {
            return;
        }
        this.chatStore.modelStore = normalizeModelStore({
            entries: normalizeModelStore(this.chatStore.modelStore).entries.filter((item) => item.id !== modelId),
            updatedAt: nowIso(),
        });
        this.chatStore.reasoningTemplates = normalizeReasoningTemplates(this.chatStore.reasoningTemplates, this.chatStore.modelStore);
        this.state.status = `Removed model entry: ${existing.label}`;
        await this.saveAndRefresh();
    }
    pushTrace(label) {
        const items = [...this.state.trace, label].slice(-6);
        this.state.trace = items;
        this.refresh();
    }
    async ask(root, text, mode = "ask", modelProfile = "balanced") {
        logChannelInfo(this.channel, "Received ask request", {
            mode,
            modelProfile,
            promptChars: String(text || "").length,
        });
        const session = this.activeSession;
        const createdAt = nowIso();
        const userMessage = sanitizeMessage({
            id: makeId("msg"),
            role: "user",
            text,
            format: "markdown",
            createdAt,
        });
        session.messages.push(userMessage);
        session.updatedAt = createdAt;
        if (session.messages.filter((message) => message.role === "user").length === 1) {
            session.title = summarizeSessionTitle(text);
        }
        const normalizedMode = ["ask", "agent", "plan"].includes(String(mode || "").toLowerCase())
            ? String(mode).toLowerCase()
            : (this.chatStore.composerMode || "ask");
        this.chatStore.composerMode = normalizedMode;
        this.state.composerMode = normalizedMode;
        const normalizedProfile = normalizeModelProfile(modelProfile || this.chatStore.modelProfile);
        this.chatStore.modelProfile = normalizedProfile;
        this.state.modelProfile = normalizedProfile;
        const commandConfirmation = getCliConfirmationRequest(text, normalizedMode);
        if (commandConfirmation) {
            const assistantMessage = sanitizeMessage({
                id: makeId("msg"),
                role: "assistant",
                text: `${commandConfirmation.title}\n\nI’ll wait for your approval before running this command.`,
                format: "markdown",
                createdAt: nowIso(),
                pendingAction: { ...commandConfirmation, modelProfile: normalizedProfile },
            });
            session.messages.push(assistantMessage);
            session.updatedAt = nowIso();
            this.state.trace = [];
            this.state.status = "Awaiting command confirmation";
            this.state.isBusy = false;
            await this.saveAndRefresh();
            this.postToSurfaces({ type: "done" });
            return;
        }
        this.state.trace = [];
        this.state.status = `Routing ${normalizedMode} request...`;
        this.state.isBusy = true;
        await this.saveAndRefresh();
        const assistantMessage = sanitizeMessage({
            id: makeId("msg"),
            role: "assistant",
            text: "",
            format: "markdown",
            createdAt: nowIso(),
            liveStatus: summarizeRequestActivity("", normalizedMode),
            liveTrace: [],
        });
        session.messages.push(assistantMessage);
        await this.saveAndRefresh();
        try {
            const reasoningTemplate = templateById(this.chatStore.reasoningTemplates, this.chatStore.activeTemplateId);
            const result = await executeDorkpipeRequest(root, session, text, {
                onEvent: (label) => {
                    this.pushTrace(label);
                    assistantMessage.liveStatus = summarizeRequestActivity(label, normalizedMode);
                    assistantMessage.liveTrace = pushRecentUnique(assistantMessage.liveTrace || [], label, 4);
                    this.refresh();
                },
                onEventDetail: (event) => {
                    const label = describeRequestEvent(event?.display_text, event?.metadata || {});
                    if (!label) {
                        return;
                    }
                    this.pushTrace(label);
                    assistantMessage.liveStatus = summarizeRequestActivity(event?.display_text || label, normalizedMode);
                    assistantMessage.liveTrace = pushRecentUnique(assistantMessage.liveTrace || [], label, 4);
                    this.refresh();
                },
                onToken: (_piece, fullText) => {
                    assistantMessage.text = fullText;
                    assistantMessage.liveStatus = "Generating a response";
                    this.refresh();
                },
                channel: this.channel,
                mode: normalizedMode,
                modelProfile: normalizedProfile,
                reasoningTemplate,
            });
            assistantMessage.liveStatus = "";
            assistantMessage.liveTrace = [];
            assistantMessage.text = result.text;
            assistantMessage.format = result.format || "markdown";
            assistantMessage.run = null;
            assistantMessage.diffPreview = "";
            session.updatedAt = nowIso();
            this.state.status = result.status;
            if (result.readyToApply?.artifact_dir) {
                const diffPreview = await readPatchPreview(root, result.readyToApply);
                const helperScriptPreview = await readHelperScriptPreview(root, result.readyToApply);
                const runDetails = await readRunInspectionData(root, result.readyToApply, {
                    state: "prepared",
                    validationStatus: result.metadata?.validation_status || "patch_applies",
                });
                assistantMessage.diffPreview = diffPreview;
                assistantMessage.run = runDetails;
                const pendingAction = sanitizePendingAction({
                    kind: "edit",
                    title: "Review edit",
                    artifactDir: result.readyToApply.artifact_dir,
                    patchPath: result.readyToApply.patch_path,
                    tracePath: result.readyToApply.trace_path,
                    artifactVersion: result.readyToApply.artifact_version,
                    structuredEditCount: result.readyToApply.structured_edit_count,
                    structuredEditTypes: result.readyToApply.structured_edit_types,
                    diffPreview,
                    helperScriptPath: result.readyToApply.helper_script_path,
                    helperScriptPurpose: result.readyToApply.helper_script_purpose,
                    helperScriptRuntime: result.readyToApply.helper_script_runtime,
                    helperScriptPreview,
                    targetFiles: result.readyToApply.target_files,
                });
                assistantMessage.pendingAction = pendingAction;
                assistantMessage.text = buildPreparedEditMessage(result.readyToApply, pendingAction);
                if (this.chatStore.autoApplyEdits) {
                    this.pushTrace("Applying confirmed edit");
                    assistantMessage.liveStatus = "Applying the change";
                    assistantMessage.liveTrace = ["Prepared the edit", "Applying confirmed edit"];
                    this.refresh();
                    const applied = await applyPreparedEdit(root, result.readyToApply.artifact_dir, {
                        onEvent: (label) => this.pushTrace(label),
                        channel: this.channel,
                    });
                    assistantMessage.liveStatus = "";
                    assistantMessage.liveTrace = [];
                    assistantMessage.text = `${result.text}\n\n---\n\n${applied.text}`;
                    assistantMessage.format = applied.format || "markdown";
                    assistantMessage.pendingAction = null;
                    assistantMessage.run = await readRunInspectionData(root, result.readyToApply, {
                        state: "applied",
                        validationStatus: applied.metadata?.validation_status,
                        applyMode: applied.metadata?.apply_mode,
                    });
                    this.state.status = applied.status;
                }
                else {
                    this.state.status = "Edit prepared; review diff below";
                }
            }
            if (!assistantMessage.run && result.metadata?.artifact_dir) {
                assistantMessage.run = await readRunInspectionData(root, result.metadata, {
                    state: result.metadata?.route === "chat" ? "completed" : "prepared",
                    validationStatus: result.metadata?.validation_status,
                });
            }
        }
        catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            assistantMessage.liveStatus = "";
            assistantMessage.liveTrace = [];
            assistantMessage.text = `DorkPipe error: ${message}`;
            assistantMessage.format = "markdown";
            this.state.status = buildRequestErrorStatus(message);
            this.channel.error(message);
        }
        session.messages = session.messages.slice(-MAX_HISTORY_MESSAGES * 4);
        this.state.isBusy = false;
        await this.saveAndRefresh();
        this.postToSurfaces({ type: "done" });
    }
    async resolvePendingAction(root, messageId, decision) {
        const session = this.activeSession;
        const assistantMessage = session.messages.find((message) => message.id === messageId);
        if (!assistantMessage?.pendingAction) {
            return;
        }
        const pending = assistantMessage.pendingAction;
        if (decision !== "approve") {
            assistantMessage.pendingAction = null;
            if (assistantMessage.run) {
                assistantMessage.run.state = "dismissed";
            }
            assistantMessage.text = `${assistantMessage.text}\n\n_${pending.kind === "edit" ? "Edit" : "Command"} dismissed._`;
            this.state.status = pending.kind === "edit" ? "Edit dismissed" : "Command dismissed";
            await this.saveAndRefresh();
            return;
        }
        this.state.isBusy = true;
        this.state.trace = [];
        this.state.status = pending.kind === "edit" ? "Applying prepared edit..." : "Running confirmed command...";
        await this.saveAndRefresh();
        try {
            if (pending.kind === "edit") {
                this.pushTrace("Applying confirmed edit");
                assistantMessage.liveStatus = "Applying the change";
                assistantMessage.liveTrace = ["Applying confirmed edit"];
                this.refresh();
                const applied = await applyPreparedEdit(root, pending.artifactDir, {
                    onEvent: (label) => {
                        this.pushTrace(label);
                        assistantMessage.liveStatus = summarizeRequestActivity(label, pending.mode);
                        assistantMessage.liveTrace = pushRecentUnique(assistantMessage.liveTrace || [], label, 4);
                        this.refresh();
                    },
                    channel: this.channel,
                });
                assistantMessage.liveStatus = "";
                assistantMessage.liveTrace = [];
                assistantMessage.text = `${assistantMessage.text}\n\n---\n\n${applied.text}`;
                assistantMessage.format = applied.format || "markdown";
                assistantMessage.pendingAction = null;
                assistantMessage.run = await readRunInspectionData(root, pending, {
                    state: "applied",
                    validationStatus: applied.metadata?.validation_status,
                    applyMode: applied.metadata?.apply_mode,
                });
                this.state.status = applied.status;
            }
            else {
                this.pushTrace("Running confirmed command");
                assistantMessage.liveStatus = "Running the command";
                assistantMessage.liveTrace = ["Running confirmed command"];
                this.refresh();
                launchTerminalCommand(root, summarizeCommandTitle(pending.requestText), pending.requestText);
                assistantMessage.liveStatus = "";
                assistantMessage.liveTrace = [];
                assistantMessage.text = `Started \`${pending.requestText}\` in a terminal.`;
                assistantMessage.format = "markdown";
                assistantMessage.pendingAction = null;
                this.state.status = "Command started in terminal";
            }
        }
        catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            assistantMessage.liveStatus = "";
            assistantMessage.liveTrace = [];
            assistantMessage.text = `DorkPipe error: ${message}`;
            assistantMessage.format = "markdown";
            assistantMessage.pendingAction = null;
            this.state.status = buildRequestErrorStatus(message);
            this.channel.error(message);
        }
        session.updatedAt = nowIso();
        this.state.isBusy = false;
        await this.saveAndRefresh();
        this.postToSurfaces({ type: "done" });
    }
    bindWebviewSurface(surfaceId, webview) {
        configureChatWebview(webview, this.context.extensionUri);
        webview.onDidReceiveMessage(async (msg) => {
            try {
                logChannelInfo(this.channel, "Webview message received", {
                    surfaceId,
                    type: msg?.type || "",
                    hasText: !!msg?.text,
                    messageId: msg?.messageId || "",
                });
                const workspaceRoot = getWorkspaceRoot();
                if (msg?.type === "switchSession" && msg.sessionId) {
                    await this.switchSession(msg.sessionId);
                    return;
                }
                if (msg?.type === "newSession") {
                    await this.newSession(msg.seed || "");
                    return;
                }
                if (msg?.type === "clearSession") {
                    await this.clearActiveSession();
                    return;
                }
                if (msg?.type === "setComposerMode") {
                    await this.setComposerMode(msg.mode);
                    return;
                }
                if (msg?.type === "setAutoApplyEdits") {
                    await this.setAutoApplyEdits(!!msg.value);
                    return;
                }
                if (msg?.type === "setModelProfile") {
                    await this.setModelProfile(msg.value);
                    return;
                }
                if (msg?.type === "setActiveTemplate") {
                    await this.setActiveTemplate(msg.templateId);
                    return;
                }
                if (msg?.type === "createTemplate") {
                    await this.createTemplate(msg.templateId);
                    return;
                }
                if (msg?.type === "saveTemplate" && msg.template) {
                    await this.saveTemplate(msg.template);
                    return;
                }
                if (msg?.type === "deleteTemplate" && msg.templateId) {
                    await this.deleteTemplate(msg.templateId);
                    return;
                }
                if (msg?.type === "upsertModelEntry" && msg.entry) {
                    await this.upsertModelEntry(msg.entry);
                    return;
                }
                if (msg?.type === "deleteModelEntry" && msg.modelId) {
                    await this.deleteModelEntry(msg.modelId);
                    return;
                }
                if (msg?.type === "clientError") {
                    const detail = msg.message ? ` (${msg.message})` : "";
                    this.channel.error(`Pipeon webview ${msg.kind || "error"}${detail}`);
                    this.state.isBusy = false;
                    this.state.status = "DorkPipe UI hit a client-side error";
                    this.state.trace = [];
                    return;
                }
                if (msg?.type === "diag") {
                    logChannelInfo(this.channel, `Webview diag: ${msg.stage || "unknown"}`, msg.extra || undefined);
                    return;
                }
                if (msg?.type === "webviewReady") {
                    const surface = this.surfaces.get(surfaceId);
                    if (surface) {
                        surface.shellVersion = String(msg.shellVersion || "");
                    }
                    logChannelInfo(this.channel, "Webview reported ready", {
                        surfaceId,
                        shellVersion: surface?.shellVersion || "unknown",
                    });
                    if ((surface?.shellVersion || "") !== CHAT_WEBVIEW_SHELL_VERSION) {
                        logChannelInfo(this.channel, "Webview shell version mismatch detected", {
                            surfaceId,
                            expected: CHAT_WEBVIEW_SHELL_VERSION,
                            actual: surface?.shellVersion || "missing",
                        });
                        this.state.status = "DorkPipe is using a stale chat shell. Settings may be unavailable until the shell is reset, but chat state will continue loading.";
                        this.state.trace = [];
                        this.state.isBusy = false;
                    }
                    this.refresh();
                    return;
                }
                if (msg?.type === "openReasoningStudio") {
                    logChannelInfo(this.channel, "Opening reasoning studio", {
                        surfaceId,
                        mode: msg.mode || "settings",
                    });
                    const nextMode = msg.mode === "models" ? "models" : msg.mode === "template" ? "template" : "settings";
                    let targetSurfaceId = surfaceId;
                    if (surfaceId === "sidebar") {
                        this.openDetachedChatPanel();
                        targetSurfaceId = this.detachedSurfaceId || surfaceId;
                    }
                    this.postToSurface(targetSurfaceId, {
                        type: "forceOpenSettings",
                        mode: nextMode,
                    });
                    return;
                }
                if (msg?.type === "canaryReady") {
                    logChannelInfo(this.channel, "Webview canary reported ready");
                    webview.postMessage({ type: "pong" });
                    return;
                }
                if (msg?.type === "ping") {
                    logChannelInfo(this.channel, "Webview canary ping received");
                    webview.postMessage({ type: "pong" });
                    return;
                }
                if (!workspaceRoot) {
                    vscode.window.showWarningMessage("DorkPipe: open a workspace folder first.");
                    return;
                }
                if (msg?.type === "resolvePendingAction" && msg.messageId) {
                    await this.resolvePendingAction(workspaceRoot, msg.messageId, msg.decision);
                    return;
                }
                if (msg?.type === "ask" && msg.text) {
                    await this.ask(workspaceRoot, msg.text, msg.mode, msg.modelProfile);
                }
            }
            catch (err) {
                const message = err instanceof Error ? err.message : String(err);
                this.channel.error(message);
                this.state.isBusy = false;
                this.state.status = buildRequestErrorStatus(message);
                this.state.trace = [];
                this.refresh();
                vscode.window.showErrorMessage(`DorkPipe: ${message}`);
            }
        });
    }
    attachWebviewSurface(surfaceId, webview) {
        this.surfaces.set(surfaceId, {
            webview,
            rendered: false,
            shellVersion: "",
        });
        this.bindWebviewSurface(surfaceId, webview);
    }
    openDetachedChatPanel() {
        if (this.detachedPanel) {
            try {
                this.detachedPanel.reveal(vscode.ViewColumn.Beside, true);
                return this.detachedPanel;
            }
            catch {
                this.detachedPanel = null;
            }
        }
        const panel = vscode.window.createWebviewPanel("pipeon.chatPanel", "DorkPipe Chat", vscode.ViewColumn.Beside, {
            enableScripts: true,
            retainContextWhenHidden: false,
            localResourceRoots: [
                vscode.Uri.joinPath(this.context.extensionUri, "webview"),
                vscode.Uri.joinPath(this.context.extensionUri, "images"),
            ],
        });
        const surfaceId = `panel:${Date.now().toString(36)}`;
        this.detachedPanel = panel;
        this.detachedSurfaceId = surfaceId;
        this.attachWebviewSurface(surfaceId, panel.webview);
        this.refresh();
        panel.onDidDispose(() => {
            logChannelInfo(this.channel, "Detached chat panel disposed", { surfaceId });
            this.surfaces.delete(surfaceId);
            if (this.detachedPanel === panel) {
                this.detachedPanel = null;
            }
            if (this.detachedSurfaceId === surfaceId) {
                this.detachedSurfaceId = "";
            }
        });
        return panel;
    }
    resolveWebviewView(webviewView) {
        this.view = webviewView;
        logChannelInfo(this.channel, "resolveWebviewView called");
        this.attachWebviewSurface("sidebar", webviewView.webview);
        const root = getWorkspaceRoot();
        if (root) {
            const host = process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST;
            const model = process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL;
            this.state.status = `Model: ${model}  |  Ollama: ${host}`;
            logChannelInfo(this.channel, "Workspace root detected for chat view", { root, model, host });
        }
        else {
            this.state.status = "Open a workspace folder to chat with DorkPipe.";
            logChannelInfo(this.channel, "No workspace root available for chat view");
        }
        this.refresh();
        webviewView.onDidDispose(() => {
            logChannelInfo(this.channel, "Chat webview disposed");
            this.view = null;
            this.surfaces.delete("sidebar");
        });
    }
}
/** @param {vscode.ExtensionContext} context */
function activate(context) {
    const channel = vscode.window.createOutputChannel("DorkPipe", { log: true });
    logChannelInfo(channel, "Activating Pipeon VS Code extension");
    const extensionVersion = context.extension?.packageJSON?.version || "unknown";
    logChannelInfo(channel, "Pipeon extension version", { version: extensionVersion });
    const chatProvider = new PipeonChatViewProvider(context, channel, extensionVersion);
    context.subscriptions.push(vscode.window.registerWebviewViewProvider(CHAT_VIEW_ID, chatProvider, {
        webviewOptions: { retainContextWhenHidden: true },
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.openChat", async () => {
        chatProvider.openDetachedChatPanel();
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.openDetachedChat", async () => {
        chatProvider.openDetachedChatPanel();
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.newChat", async () => {
        await chatProvider.newSession();
        chatProvider.openDetachedChatPanel();
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.clearChat", async () => {
        await chatProvider.clearActiveSession();
        chatProvider.openDetachedChatPanel();
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.resetChatShell", async () => {
        await chatProvider.resetWebviewShell();
        chatProvider.openDetachedChatPanel();
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.openWelcome", async () => {
        openPipeonWelcome(context);
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.openContextBundle", async () => {
        const root = getWorkspaceRoot();
        if (!root) {
            vscode.window.showWarningMessage("DorkPipe: open a workspace folder first.");
            return;
        }
        try {
            const ctxPath = await resolveContextBundlePath(root);
            if (!ctxPath) {
                throw new Error("missing");
            }
            const text = await fs.readFile(ctxPath, "utf8");
            channel.clear();
            channel.appendLine(text);
            channel.show(true);
        }
        catch {
            vscode.window.showInformationMessage("DorkPipe: no legacy compatibility snapshot found.");
        }
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.openWebviewCanary", async () => {
        const panel = vscode.window.createWebviewPanel("pipeon.webviewCanary", "Pipeon Webview Canary", vscode.ViewColumn.Active, {
            enableScripts: true,
            localResourceRoots: [vscode.Uri.joinPath(context.extensionUri, "webview")],
            retainContextWhenHidden: true,
        });
        panel.webview.html = renderCanaryHtml(panel.webview, context.extensionUri);
        panel.webview.onDidReceiveMessage((msg) => {
            if (msg?.type === "canaryReady") {
                logChannelInfo(channel, "Canary webview reported ready");
                panel.webview.postMessage({ type: "pong" });
                return;
            }
            if (msg?.type === "ping") {
                logChannelInfo(channel, "Canary webview ping received");
                panel.webview.postMessage({ type: "pong" });
            }
        });
    }));
    context.subscriptions.push(vscode.commands.registerCommand("pipeon.showReadme", async () => {
        const root = vscode.workspace.workspaceFolders?.[0]?.uri;
        if (!root) {
            vscode.window.showWarningMessage("DorkPipe: open the dockpipe repository as a workspace folder first.");
            return;
        }
        const doc = vscode.Uri.joinPath(root, "packages", "pipeon", "resolvers", "pipeon", "assets", "docs", "pipeon-vscode-fork.md");
        try {
            await vscode.workspace.fs.stat(doc);
            const docShow = await vscode.workspace.openTextDocument(doc);
            await vscode.window.showTextDocument(docShow, { preview: true });
        }
        catch {
            vscode.window.showInformationMessage("DorkPipe: fork doc not found in this workspace — open the dockpipe repo root.");
        }
    }));
    context.subscriptions.push(vscode.window.tabGroups.onDidChangeTabs(() => {
        setTimeout(() => {
            void redirectStockWelcomeToPipeon(context);
        }, 50);
    }));
    setTimeout(() => {
        void replaceStockWelcomeWithPipeon(context);
        void resetPanelToBottomOnce(context);
    }, 400);
}
function deactivate() { }
module.exports = { activate, deactivate };
