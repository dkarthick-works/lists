// Access tokens live in memory only; the HttpOnly refresh cookie (managed by
// Goauth through the /api/auth proxy) restores the session after a reload.
let accessToken: string | null = null;
let onSignedOut: () => void = () => {};

export function setSignedOutHandler(fn: () => void) {
  onSignedOut = fn;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function parse(res: Response) {
  const body = res.status === 204 ? null : await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, body?.error ?? `Request failed (${res.status})`);
  return body;
}

function post(path: string, body?: unknown) {
  return fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
}

// Refresh tokens rotate on every use, so concurrent callers must share one request.
let refreshing: Promise<boolean> | null = null;

export function refresh(): Promise<boolean> {
  refreshing ??= post("/api/auth/refresh")
    .then(parse)
    .then((b) => {
      accessToken = b.access_token;
      return true;
    })
    .catch(() => {
      accessToken = null;
      return false;
    })
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

export async function login(email: string, password: string) {
  const b = await parse(await post("/api/auth/login", { email, password }));
  accessToken = b.access_token;
}

export async function logout() {
  accessToken = null;
  await post("/api/auth/logout").catch(() => {});
}

export const signup = (email: string, password: string) =>
  post("/api/auth/signup", { email, password }).then(parse);

export const forgotPassword = (email: string) =>
  post("/api/auth/forgot-password", { email }).then(parse);

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const send = () =>
    fetch(path, {
      method,
      headers: {
        Authorization: `Bearer ${accessToken}`,
        ...(body ? { "Content-Type": "application/json" } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });

  let res = await send();
  if (res.status === 401) {
    if (!(await refresh())) {
      onSignedOut();
      throw new ApiError(401, "Session expired");
    }
    res = await send();
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
  makeList: (id: string) => request<Item>("PATCH", `/api/items/${id}`, { is_list: true }),
  remove: (id: string) => request<null>("DELETE", `/api/items/${id}`),
};
