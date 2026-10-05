// Session handling. Access tokens live in memory only; the HttpOnly refresh
// cookie (managed by Goauth through the /api/auth proxy) restores the session
// after a reload.
//
// Goauth rotates the refresh token on every use, so a second refresh sent with
// the same cookie fails and would look like a logout. Everything here exists to
// make sure that never happens by accident:
//  - one refresh at a time per page (single-flight) and per browser (Web Lock),
//    with the result shared to other tabs / PWA windows over a BroadcastChannel;
//  - only an explicit rejection from Goauth ends the session — being offline or
//    hitting a 5xx leaves it intact to retry later.

export type AuthState = "in" | "out";
export type RefreshResult = "ok" | "expired" | "unavailable";

let accessToken: string | null = null;
// Bumped on logout so a refresh that was already in flight cannot sign back in.
let generation = 0;
let lastRemoteTokenAt = 0;
let onAuthChange: (state: AuthState) => void = () => {};

export function setAuthChangeHandler(fn: (state: AuthState) => void) {
  onAuthChange = fn;
}

type AuthMessage = { type: "token"; token: string } | { type: "logout" };

const channel = typeof BroadcastChannel === "undefined" ? null : new BroadcastChannel("lists_auth");
channel?.addEventListener("message", (e: MessageEvent<AuthMessage>) => {
  if (e.data.type === "token") {
    accessToken = e.data.token;
    lastRemoteTokenAt = Date.now();
    onAuthChange("in");
  } else {
    generation += 1;
    accessToken = null;
    onAuthChange("out");
  }
});

function adopt(token: string) {
  accessToken = token;
  channel?.postMessage({ type: "token", token } satisfies AuthMessage);
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

const UNREACHABLE = "Can't reach the server. Check your connection and try again.";

async function parse(res: Response) {
  const body = res.status === 204 ? null : await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, body?.error ?? `Request failed (${res.status})`);
  return body;
}

// fetch that reports a dropped connection as an ApiError (status 0) rather
// than the browser's "Failed to fetch".
function send(path: string, init: RequestInit) {
  return fetch(path, { credentials: "same-origin", ...init }).catch(() => {
    throw new ApiError(0, UNREACHABLE);
  });
}

function post(path: string, body?: unknown) {
  return send(path, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
}

async function requestRefresh(): Promise<RefreshResult> {
  const startedGeneration = generation;
  let res: Response;
  try {
    res = await post("/api/auth/refresh");
  } catch {
    return "unavailable";
  }
  if (!res.ok) {
    // Goauth answers 400/401 when the cookie is missing, used or expired.
    // Anything else (proxy 502, 5xx, 429) says nothing about the session.
    return [400, 401, 403].includes(res.status) ? "expired" : "unavailable";
  }
  const token = (await res.json().catch(() => null))?.access_token;
  if (typeof token !== "string") return "unavailable";
  if (startedGeneration !== generation) return "expired";
  adopt(token);
  return "ok";
}

async function coordinatedRefresh(): Promise<RefreshResult> {
  if (!navigator.locks) return requestRefresh();
  const startedAt = Date.now();
  return navigator.locks.request("lists_refresh", async () => {
    // If we had to queue, another tab was refreshing: its token reaches us by
    // broadcast just before it releases the lock, so give it a moment to land
    // and reuse it instead of spending the rotated cookie a second time.
    if (Date.now() - startedAt > 20) {
      await new Promise((r) => setTimeout(r, 100));
      if (accessToken && lastRemoteTokenAt >= startedAt) return "ok";
    }
    return requestRefresh();
  });
}

let refreshing: Promise<RefreshResult> | null = null;

export function refresh(): Promise<RefreshResult> {
  refreshing ??= coordinatedRefresh()
    .catch((): RefreshResult => "unavailable")
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

export async function login(email: string, password: string) {
  const b = await parse(await post("/api/auth/login", { email, password }));
  adopt(b.access_token);
}

export async function logout() {
  generation += 1;
  accessToken = null;
  channel?.postMessage({ type: "logout" } satisfies AuthMessage);
  // Let any in-flight refresh settle first, so Goauth revokes the newest cookie.
  await refreshing;
  await post("/api/auth/logout").catch(() => {});
}

export const signup = (email: string, password: string) =>
  post("/api/auth/signup", { email, password }).then(parse);

export const forgotPassword = (email: string) =>
  post("/api/auth/forgot-password", { email }).then(parse);

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const attempt = () =>
    send(path, {
      method,
      headers: {
        Authorization: `Bearer ${accessToken}`,
        ...(body ? { "Content-Type": "application/json" } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });

  let res = await attempt();
  if (res.status === 401) {
    const result = await refresh();
    if (result === "expired") {
      onAuthChange("out");
      throw new ApiError(401, "Session expired");
    }
    if (result === "unavailable") throw new ApiError(0, UNREACHABLE);
    res = await attempt();
  }
  return parse(res);
}

export const me = () => request<{ user_id: string; email: string }>("GET", "/api/auth/me");

export interface Item {
  id: string;
  parent_id: string | null;
  text: string;
  is_list: boolean;
  updated_at: string;
  pinned_at: string | null;
  completed_at: string | null;
}

export interface Suggestion {
  id: string;
  text: string;
  parent_text: string | null;
}

export interface ListDetail {
  list: Item;
  entries: Item[];
  ancestors: { id: string; text: string }[];
}

export const lists = {
  all: () => request<Item[]>("GET", "/api/lists"),
  recent: () => request<Item[]>("GET", "/api/lists/recent"),
  autocomplete: (q: string) => request<Suggestion[]>("GET", `/api/lists/autocomplete?q=${encodeURIComponent(q)}`),
  create: (title: string) => request<Item>("POST", "/api/lists", { title }),
  get: (id: string) => request<ListDetail>("GET", `/api/lists/${id}`),
  addEntry: (listId: string, text: string, isList: boolean) =>
    request<Item>("POST", `/api/lists/${listId}/entries`, { text, is_list: isList }),
  rename: (id: string, text: string) => request<Item>("PATCH", `/api/items/${id}`, { text }),
  setPinned: (id: string, pinned: boolean) => request<Item>("PUT", `/api/lists/${id}/pin`, { pinned }),
  setCompleted: (id: string, completed: boolean) =>
    request<Item>("PUT", `/api/items/${id}/completed`, { completed }),
  reorder: (listId: string, ids: string[]) => request<null>("PUT", `/api/lists/${listId}/order`, { ids }),
  makeList: (id: string) => request<Item>("PATCH", `/api/items/${id}`, { is_list: true }),
  remove: (id: string) => request<null>("DELETE", `/api/items/${id}`),
};
