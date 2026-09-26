"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { AnswerText } from "@/components/answer-text";
import { Button, ErrorState, Skeleton } from "@/components/ui";
import { api, ApiError, formatDate, streamChat, type ChatMessage, type Citation } from "@/lib/api";
import { useAsync } from "@/lib/use-async";

type Turn = {
  key: string;
  question: string;
  answer: string;
  citations: Citation[];
  status: "streaming" | "done" | "error";
  error?: ApiError;
};

const SUGGESTIONS = [
  "Skill apa yang paling banyak diminta untuk AI Engineer?",
  "Dengan skill saya sekarang, apa yang sebaiknya saya pelajari dulu?",
  "Rekomendasikan sumber belajar gratis untuk memulai RAG.",
];

const SECTION_LABEL: Record<string, string> = {
  overview: "RINGKASAN",
  about: "TENTANG PERUSAHAAN",
  responsibilities: "TANGGUNG JAWAB",
  qualifications: "KUALIFIKASI",
  benefits: "BENEFIT",
};

function turnsFromHistory(messages: ChatMessage[]): Turn[] {
  const turns: Turn[] = [];
  for (const m of messages) {
    if (m.role === "user") turns.push({ key: m.id, question: m.content, answer: "", citations: [], status: "done" });
    else if (turns.length && !turns[turns.length - 1].answer) {
      const t = turns[turns.length - 1];
      t.answer = m.content;
      t.citations = m.citations ?? [];
    }
  }
  return turns.filter((t) => t.answer);
}

export default function ChatPage() {
  const history = useAsync(() => api.chatHistory(), []);
  const context = useAsync(() => Promise.all([api.profile().catch(() => null), api.roadmap().catch(() => null)]), []);
  const [live, setLive] = useState<Turn[]>([]);
  const [input, setInput] = useState("");
  const [focus, setFocus] = useState<{ turn: string; n: number | null } | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const seqRef = useRef(0);
  const endRef = useRef<HTMLDivElement>(null);

  const turns = [...turnsFromHistory(history.data ?? []), ...live];
  const streaming = live.some((t) => t.status === "streaming");
  // Panel sumber menampilkan jawaban yang sedang difokus, atau jawaban terakhir.
  const focusTurn = turns.find((t) => t.key === focus?.turn) ?? turns[turns.length - 1];

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [turns.length, live]);

  useEffect(() => () => abortRef.current?.abort(), []);

  function update(key: string, patch: Partial<Turn> | ((t: Turn) => Partial<Turn>)) {
    setLive((ts) => ts.map((t) => (t.key === key ? { ...t, ...(typeof patch === "function" ? patch(t) : patch) } : t)));
  }

  async function ask(question: string) {
    const q = question.trim();
    if (!q || streaming) return;
    const key = `live-${++seqRef.current}`;
    setLive((ts) => [...ts, { key, question: q, answer: "", citations: [], status: "streaming" }]);
    setInput("");
    setFocus({ turn: key, n: null });

    const ctrl = new AbortController();
    abortRef.current = ctrl;
    await streamChat(
      q,
      {
        onToken: (text) => update(key, (t) => ({ answer: t.answer + text })),
        onDone: (done) => update(key, { answer: done.answer, citations: done.citations, status: "done" }),
        onError: (error) => update(key, { status: "error", error }),
      },
      ctrl.signal,
    );
    if (ctrl.signal.aborted) update(key, (t) => (t.status === "streaming" ? { status: "done" } : {}));
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    void ask(input);
  }

  function cite(turn: Turn, n: number) {
    setFocus({ turn: turn.key, n });
    document.getElementById(`src-${n}`)?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }

  const [profile, roadmap] = context.data ?? [null, null];

  return (
    <main className="flex min-h-screen flex-col lg:h-screen lg:flex-row">
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex min-h-16 shrink-0 flex-wrap items-center justify-between gap-2 border-b border-line px-6 py-3 md:px-10">
          <h1 className="m-0 text-[16px] font-semibold">Tanya soal karier AI</h1>
          <div className="text-[13px] text-muted">
            {profile ? (
              <>
                Konteks: profil kamu · target {profile.target_role?.name ?? "-"}
                {roadmap && ` · roadmap v${roadmap.version}`}
              </>
            ) : (
              <>
                Tanpa profil · <Link href="/">isi profil</Link> agar jawaban lebih personal
              </>
            )}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto">
          <div className="mx-auto flex w-full max-w-[800px] flex-col gap-8 px-6 py-10 md:px-10">
            {history.error ? (
              <ErrorState error={history.error} onRetry={history.reload} title="Riwayat chat gagal dimuat" />
            ) : history.loading ? (
              <div className="flex flex-col gap-4">
                <Skeleton className="ml-auto h-12 w-2/3" />
                <Skeleton className="h-32" />
              </div>
            ) : turns.length === 0 ? (
              <div className="flex flex-col gap-4">
                <p className="m-0 text-[15px] leading-6 text-ink-2">
                  Tanyakan skill, urutan belajar, lowongan, atau sumber belajar. Jawaban hanya bersumber dari data lowongan
                  dan sumber belajar terkurasi, lengkap dengan sitasi.
                </p>
                <div className="flex flex-col items-start gap-2">
                  {SUGGESTIONS.map((s) => (
                    <button
                      key={s}
                      type="button"
                      onClick={() => ask(s)}
                      className="rounded-control border border-line-strong bg-surface px-3 py-2 text-left text-[14px] text-ink-2 hover:border-ink hover:text-ink"
                    >
                      {s}
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              turns.map((t) => (
                <div key={t.key} className="flex flex-col gap-6">
                  <div className="max-w-[560px] self-end rounded-panel border border-line bg-surface px-4 py-3 text-[15px] leading-6">
                    {t.question}
                  </div>
                  <article className="flex flex-col gap-4" aria-busy={t.status === "streaming"}>
                    <div className="eyebrow">
                      {t.status === "streaming"
                        ? "Menyusun jawaban…"
                        : t.status === "error"
                          ? "Jawaban gagal"
                          : `Jawaban · ${t.citations.length ? `${t.citations.length} sumber` : "tanpa sitasi"}`}
                    </div>
                    {t.answer ? (
                      <AnswerText text={t.answer} active={focusTurn?.key === t.key ? focus?.n : null} onCite={(n) => cite(t, n)} />
                    ) : (
                      t.status === "streaming" && <Skeleton className="h-5 w-1/2" />
                    )}
                    {t.status === "error" && t.error && <ChatError error={t.error} onRetry={() => ask(t.question)} />}
                    {t.status === "done" && t.answer && <CopyButton text={t.answer} />}
                  </article>
                </div>
              ))
            )}
            <div ref={endRef} />
          </div>
        </div>

        <form onSubmit={onSubmit} className="mx-auto w-full max-w-[880px] shrink-0 px-6 pt-4 pb-6 md:px-10">
          <label htmlFor="q" className="sr-only">
            Pertanyaan
          </label>
          <div className="flex items-center gap-2 rounded-panel border border-line-strong bg-surface py-2 pr-2 pl-4 focus-within:border-compass focus-within:ring-[3px] focus-within:ring-compass-soft">
            <input
              id="q"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              maxLength={2000}
              autoComplete="off"
              placeholder="Tanyakan skill, urutan belajar, atau lowongan…"
              className="h-8 min-w-0 flex-1 border-0 bg-transparent text-[15px] text-ink outline-none"
            />
            {streaming ? (
              <Button type="button" onClick={() => abortRef.current?.abort()}>
                Hentikan
              </Button>
            ) : (
              <Button type="submit" variant="primary" disabled={!input.trim()}>
                Kirim
              </Button>
            )}
          </div>
          <div className="pt-2 text-[12px] text-muted">
            Jawaban hanya bersumber dari data lowongan dan sumber belajar yang dikurasi. Di luar topik karier tech akan ditolak.
          </div>
        </form>
      </div>

      <SourcesPanel turn={focusTurn} activeN={focus?.turn === focusTurn?.key ? focus?.n ?? null : null} />
    </main>
  );
}

function ChatError({ error, onRetry }: { error: ApiError; onRetry: () => void }) {
  const message =
    error.status === 429
      ? `Terlalu banyak pertanyaan dalam waktu singkat. Coba lagi${error.retryAfter ? ` dalam ${error.retryAfter} detik` : " sebentar lagi"}.`
      : error.message;
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 rounded-control bg-error-soft px-4 py-3 text-[14px] text-error">
      <span>{message}</span>
      <Button size="sm" onClick={onRetry}>
        Kirim ulang
      </Button>
    </div>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex gap-2">
      <button
        type="button"
        aria-label={copied ? "Jawaban tersalin" : "Salin jawaban"}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(text);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          } catch {
            // clipboard bisa ditolak browser; tidak fatal
          }
        }}
        className="flex h-8 items-center gap-1.5 rounded-control border border-line-strong px-2.5 text-[12px] text-ink-2 hover:bg-sunken"
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.4" aria-hidden>
          <rect x="4.5" y="4.5" width="7" height="7" rx="1" />
          <path d="M9.5 2.5h-6a1 1 0 0 0-1 1v6" />
        </svg>
        {copied ? "Tersalin" : "Salin"}
      </button>
    </div>
  );
}

function sourceKind(c: Citation) {
  if (c.source_type === "market_data") return "DATA PASAR";
  if (c.source_type === "learning_resource") return "SUMBER BELAJAR";
  return `LOWONGAN${c.section && SECTION_LABEL[c.section] ? ` · ${SECTION_LABEL[c.section]}` : ""}`;
}

function sourceMeta(c: Citation) {
  const m = c.meta ?? {};
  if (c.source_type === "market_data") {
    return `n = ${m.total_jobs ?? "-"} lowongan · snapshot ${formatDate(String(m.snapshot_date ?? ""))}`;
  }
  if (c.source_type === "learning_resource") {
    return [m.skill_name, m.type, m.is_free === false ? "berbayar" : "gratis"].filter(Boolean).join(" · ");
  }
  return [m.company, m.source_name, m.posted_date ? `diposting ${formatDate(String(m.posted_date))}` : null].filter(Boolean).join(" · ");
}

function SourcesPanel({ turn, activeN }: { turn?: Turn; activeN: number | null }) {
  const citations = turn?.citations ?? [];
  return (
    <aside aria-label="Sumber" className="flex shrink-0 flex-col border-t border-line bg-surface lg:w-[360px] lg:border-t-0 lg:border-l">
      <div className="flex h-16 shrink-0 items-center border-b border-line px-6 text-[14px] font-semibold">Sumber</div>
      <div className="flex-1 overflow-y-auto">
        {citations.length === 0 ? (
          <p className="m-0 px-6 py-5 text-[13px] leading-5 text-muted">
            {turn?.status === "streaming"
              ? "Sumber muncul setelah jawaban selesai."
              : turn?.status === "error"
                ? "Belum ada jawaban, jadi belum ada sumber."
                : turn
                  ? "Jawaban ini tidak menyitir sumber."
                  : "Sumber yang dipakai jawaban akan tampil di sini."}
          </p>
        ) : (
          citations.map((c) => (
            <div
              key={c.n}
              id={`src-${c.n}`}
              className={"flex scroll-mt-4 flex-col gap-2 border-b border-line px-6 py-5 " + (activeN === c.n ? "bg-paper" : "")}
            >
              <div className="flex items-center gap-2">
                <span className="rounded-chip border border-line-strong px-[5px] font-mono text-[12px] text-compass-strong">{c.n}</span>
                <span className="font-mono text-[11px] tracking-[0.06em] text-muted">{sourceKind(c)}</span>
              </div>
              <div className="text-[14px] leading-5 font-medium">
                {c.url ? (
                  <a href={c.url} target="_blank" rel="noopener noreferrer" className="text-ink hover:text-compass-strong">
                    {c.title ?? c.url}
                  </a>
                ) : c.source_type === "market_data" ? (
                  <Link href="/dashboard" className="text-ink hover:text-compass-strong">
                    {c.title}
                  </Link>
                ) : (
                  c.title
                )}
              </div>
              {c.excerpt && (
                <div className="border-t border-dashed border-line pt-2 text-[13px] leading-5 text-ink-2">{c.excerpt}</div>
              )}
              <div className="text-[12px] text-muted">{sourceMeta(c)}</div>
            </div>
          ))
        )}
      </div>
    </aside>
  );
}
