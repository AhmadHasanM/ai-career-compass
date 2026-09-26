// Client API backend Go. Sesi anonim disimpan di localStorage dan dikirim lewat header X-Session-Id.

export const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

const SESSION_KEY = "acc.session_id";
const DEFAULT_TIMEOUT_MS = 15_000;

// ---------- tipe (mengikuti respons JSON backend) ----------

export type Role = { id: number; name: string; slug: string };

export type Skill = {
  id: number;
  name: string;
  slug: string;
  category: string;
  description: string | null;
  aliases: string[];
  prerequisite_ids: number[];
};

export type Proficiency = "beginner" | "intermediate" | "advanced";

export type UserSkill = {
  skill_id: number;
  name: string;
  slug: string;
  category: string;
  proficiency: Proficiency;
  source: "manual" | "cv";
};

export type Profile = {
  id: string;
  education: string | null;
  current_job: string | null;
  target_role: Role | null;
  hours_per_week: number | null;
  skills: UserSkill[];
  updated_at: string;
};

export type ProfileInput = {
  education?: string | null;
  current_job?: string | null;
  target_role_id: number | null;
  hours_per_week: number | null;
  skills: { skill_id: number; proficiency: Proficiency }[];
};

export type MarketSample = { total_jobs: number; snapshot_date: string | null; small_sample: boolean };

export type DemandItem = {
  skill_id: number;
  name: string;
  slug: string;
  category: string;
  job_count: number;
  required_count: number;
  demand_pct: number;
  required_pct: number;
};

export type SkillDemand = MarketSample & { role: Role; level: string | null; items: DemandItem[] };

export type EvidenceJob = {
  id: string;
  title: string;
  company: string | null;
  level: string | null;
  location: string | null;
  work_type: string | null;
  source_name: string;
  source_url: string | null;
  posted_date: string | null;
  collected_at: string;
  requirement_type: "required" | "preferred";
};

export type EvidenceJobs = { skill: Skill; total: number; jobs: EvidenceJob[] };

export type GapItem = DemandItem & { priority_score: number };

export type Gap = MarketSample & {
  role: Role;
  threshold_pct: number;
  coverage_pct: number;
  gaps: GapItem[];
  owned: DemandItem[];
};

export type LearningResource = {
  id: string;
  skill_id: number;
  title: string;
  url: string;
  type: string;
  level: string | null;
  language: "id" | "en";
  is_free: boolean;
  est_hours: number | null;
};

export type NodeStatus = "todo" | "in_progress" | "done";

export type RoadmapNode = {
  id: string;
  skill_id: number;
  name: string;
  slug: string;
  category: string;
  order_index: number;
  stage: string;
  priority_score: number | null;
  demand_pct: number | null;
  est_weeks: number | null;
  rationale: string | null;
  status: NodeStatus;
  resources: LearningResource[];
};

export type Roadmap = {
  id: string;
  version: number;
  role: Role;
  model_name: string | null;
  generated_at: string;
  hours_per_week: number | null;
  total_weeks: number;
  nodes: RoadmapNode[];
  edges: { from_skill_id: number; to_skill_id: number }[];
};

export type Citation = {
  n: number;
  source_type: "job_posting" | "learning_resource" | "market_data";
  chunk_id: string | null;
  source_id: string | null;
  title: string | null;
  section: string | null;
  excerpt: string;
  url: string | null;
  meta: Record<string, string | number | boolean | null>;
};

export type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations: Citation[];
  created_at: string;
};

export type ChatDone = {
  answer: string;
  citations: Citation[];
  model: string | null;
  usage: { prompt_tokens: number | null; completion_tokens: number | null };
  latency_ms: number | null;
};

// ---------- error ----------

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
    public retryAfter?: number,
  ) {
    super(message);
  }
}

function friendlyNetworkError(e: unknown): ApiError {
  if (e instanceof DOMException && e.name === "AbortError") {
    return new ApiError(0, "timeout", "Server terlalu lama merespons. Coba lagi sebentar lagi.");
  }
  return new ApiError(0, "network", "Tidak bisa terhubung ke server. Pastikan backend berjalan.");
}

async function toApiError(res: Response): Promise<ApiError> {
  const retry = Number(res.headers.get("Retry-After")) || undefined;
  try {
    const body = await res.json();
    const e = body?.error ?? {};
    return new ApiError(res.status, e.code ?? "error", e.message ?? res.statusText, e.fields, retry);
  } catch {
    return new ApiError(res.status, "error", res.statusText || "Terjadi kesalahan", undefined, retry);
  }
}

// ---------- sesi ----------

function readSession(): string | null {
  try {
    return window.localStorage.getItem(SESSION_KEY);
  } catch {
    return null;
  }
}

function writeSession(id: string | null) {
  try {
    if (id) window.localStorage.setItem(SESSION_KEY, id);
    else window.localStorage.removeItem(SESSION_KEY);
  } catch {
    // localStorage bisa diblokir (mode privat); sesi tetap hidup di memori untuk tab ini.
  }
}

let memorySession: string | null = null;
let creating: Promise<string> | null = null;

async function ensureSession(): Promise<string> {
  const existing = memorySession ?? readSession();
  if (existing) return (memorySession = existing);
  // Hindari beberapa POST /api/sessions paralel saat banyak komponen memuat bersamaan.
  creating ??= (async () => {
    const res = await fetchWithTimeout(`${API_URL}/api/sessions`, { method: "POST" });
    if (!res.ok) throw await toApiError(res);
    const { session_id } = (await res.json()) as { session_id: string };
    memorySession = session_id;
    writeSession(session_id);
    return session_id;
  })().finally(() => {
    creating = null;
  });
  return creating;
}

function resetSession() {
  memorySession = null;
  writeSession(null);
}

// ---------- request ----------

async function fetchWithTimeout(url: string, init: RequestInit = {}, timeoutMs = DEFAULT_TIMEOUT_MS) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  const onAbort = () => ctrl.abort();
  init.signal?.addEventListener("abort", onAbort);
  try {
    return await fetch(url, { ...init, signal: ctrl.signal });
  } catch (e) {
    throw friendlyNetworkError(e);
  } finally {
    clearTimeout(timer);
    init.signal?.removeEventListener("abort", onAbort);
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  session?: boolean;
  timeoutMs?: number;
  signal?: AbortSignal;
};

async function request<T>(path: string, opts: RequestOptions = {}, retried = false): Promise<T> {
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.session) headers["X-Session-Id"] = await ensureSession();

  const res = await fetchWithTimeout(
    `${API_URL}${path}`,
    {
      method: opts.method ?? "GET",
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      signal: opts.signal,
    },
    opts.timeoutMs,
  );
  if (res.ok) return (await res.json()) as T;

  const err = await toApiError(res);
  // Sesi kedaluwarsa / database di-reset: buat sesi baru sekali lalu ulangi.
  if (opts.session && err.status === 401 && err.code === "session_invalid" && !retried) {
    resetSession();
    return request<T>(path, opts, true);
  }
  throw err;
}

// ---------- endpoint ----------

export const api = {
  roles: () => request<{ roles: Role[] }>("/api/roles").then((r) => r.roles),
  skills: () => request<{ skills: Skill[] }>("/api/skills").then((r) => r.skills),

  profile: async (): Promise<Profile | null> => {
    try {
      return await request<Profile>("/api/profile", { session: true });
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return null;
      throw e;
    }
  },
  saveProfile: (input: ProfileInput) => request<Profile>("/api/profile", { method: "PUT", body: input, session: true }),

  skillDemand: (params: { role?: string; level?: string; limit?: number } = {}) => {
    const q = new URLSearchParams();
    if (params.role) q.set("role", params.role);
    if (params.level) q.set("level", params.level);
    if (params.limit) q.set("limit", String(params.limit));
    return request<SkillDemand>(`/api/insights/skill-demand?${q}`);
  },
  evidenceJobs: (skillId: number, params: { role?: string; level?: string; limit?: number; offset?: number } = {}) => {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") q.set(k, String(v));
    return request<EvidenceJobs>(`/api/insights/skill-demand/${skillId}/jobs?${q}`);
  },

  gap: () => request<Gap>("/api/gap", { session: true }),

  roadmap: async (): Promise<Roadmap | null> => {
    try {
      return await request<Roadmap>("/api/roadmap", { session: true });
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return null;
      throw e;
    }
  },
  // Memanggil LLM untuk penjelasan node; backend memakai fallback bila LLM gagal (maks. ~20 detik).
  generateRoadmap: () => request<Roadmap>("/api/roadmap/generate", { method: "POST", session: true, timeoutMs: 45_000 }),

  chatHistory: () =>
    request<{ messages: ChatMessage[] }>("/api/chat/history?limit=50", { session: true }).then((r) => r.messages),
};

// ---------- chat SSE ----------

export type ChatStreamHandlers = {
  onToken: (text: string) => void;
  onDone: (done: ChatDone) => void;
  onError: (err: ApiError) => void;
};

// Batas diam: jika tidak ada event baru selama ini, stream dianggap macet.
const CHAT_IDLE_TIMEOUT_MS = 30_000;

export async function streamChat(message: string, handlers: ChatStreamHandlers, signal?: AbortSignal, retried = false) {
  let sid: string;
  try {
    sid = await ensureSession();
  } catch (e) {
    handlers.onError(e instanceof ApiError ? e : friendlyNetworkError(e));
    return;
  }

  const ctrl = new AbortController();
  signal?.addEventListener("abort", () => ctrl.abort());
  let idle = setTimeout(() => ctrl.abort(), CHAT_IDLE_TIMEOUT_MS);
  const bump = () => {
    clearTimeout(idle);
    idle = setTimeout(() => ctrl.abort(), CHAT_IDLE_TIMEOUT_MS);
  };

  let res: Response;
  try {
    res = await fetch(`${API_URL}/api/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Session-Id": sid, Accept: "text/event-stream" },
      body: JSON.stringify({ message }),
      signal: ctrl.signal,
    });
  } catch (e) {
    clearTimeout(idle);
    if (!signal?.aborted) handlers.onError(friendlyNetworkError(e));
    return;
  }

  if (!res.ok || !res.body) {
    clearTimeout(idle);
    const err = await toApiError(res);
    if (err.status === 401 && err.code === "session_invalid" && !retried) {
      resetSession();
      return streamChat(message, handlers, signal, true);
    }
    handlers.onError(err);
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let finished = false;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      bump();
      buffer += decoder.decode(value, { stream: true });
      let sep: number;
      while ((sep = buffer.indexOf("\n\n")) !== -1) {
        const block = buffer.slice(0, sep);
        buffer = buffer.slice(sep + 2);
        const ev = parseSSEBlock(block);
        if (!ev) continue;
        if (ev.event === "token") handlers.onToken((ev.data as { text: string }).text);
        else if (ev.event === "done") {
          finished = true;
          handlers.onDone(ev.data as ChatDone);
        } else if (ev.event === "error") {
          finished = true;
          const d = ev.data as { code?: string; message?: string };
          handlers.onError(new ApiError(200, d.code ?? "stream_error", d.message ?? "Jawaban gagal disusun."));
        }
      }
    }
  } catch (e) {
    if (signal?.aborted) return;
    finished = true;
    handlers.onError(
      e instanceof DOMException && e.name === "AbortError"
        ? new ApiError(0, "timeout", "Jawaban berhenti di tengah jalan. Coba kirim ulang pertanyaannya.")
        : friendlyNetworkError(e),
    );
  } finally {
    clearTimeout(idle);
  }
  if (!finished && !signal?.aborted) {
    handlers.onError(new ApiError(0, "incomplete", "Koneksi terputus sebelum jawaban selesai."));
  }
}

export function parseSSEBlock(block: string): { event: string; data: unknown } | null {
  let event = "message";
  const data: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
  }
  if (!data.length) return null;
  try {
    return { event, data: JSON.parse(data.join("\n")) };
  } catch {
    return null;
  }
}

// ---------- format ----------

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];

export function formatDate(iso: string | null | undefined, withYear = true): string {
  if (!iso) return "-";
  const [y, m, d] = iso.slice(0, 10).split("-").map(Number);
  if (!y || !m || !d) return iso;
  return `${String(d).padStart(2, "0")} ${MONTHS[m - 1]}${withYear ? ` ${y}` : ""}`;
}

export function formatPct(n: number): string {
  return `${Number.isInteger(n) ? n : n.toFixed(1)}%`;
}
