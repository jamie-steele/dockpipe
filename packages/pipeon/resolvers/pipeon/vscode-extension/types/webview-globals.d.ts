interface VsCodeWebviewApi {
  postMessage(message: unknown): void;
  getState<T = unknown>(): T | null;
  setState<T = unknown>(state: T): void;
}

declare function acquireVsCodeApi(): VsCodeWebviewApi;
