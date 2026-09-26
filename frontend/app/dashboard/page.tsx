"use client";

import Link from "next/link";
import { useState } from "react";
import {
  Button,
  DemandRow,
  EmptyState,
  ErrorState,
  PageHeader,
  Panel,
  SampleNote,
  Select,
  Skeleton,
  SmallSampleWarning,
} from "@/components/ui";
import { api, formatDate, type EvidenceJob } from "@/lib/api";
import { useAsync } from "@/lib/use-async";

const LEVELS = [
  { value: "", label: "Semua level" },
  { value: "intern", label: "Intern" },
  { value: "junior", label: "Junior" },
  { value: "mid", label: "Mid" },
  { value: "senior", label: "Senior" },
  { value: "lead", label: "Lead" },
];
const PAGE = 20;

export default function DashboardPage() {
  const [role, setRole] = useState("ai-engineer");
  const [level, setLevel] = useState("");
  const [selected, setSelected] = useState<number | null>(null);

  const roles = useAsync(() => api.roles(), []);
  const profile = useAsync(() => api.profile().catch(() => null), []);
  const demand = useAsync(() => api.skillDemand({ role, level, limit: 15 }), [role, level]);

  const owned = new Set(profile.data?.skills.map((s) => s.skill_id) ?? []);
  const items = demand.data?.items ?? [];

  // Pilihan pengguna dipakai selama skill itu masih ada di daftar; jika tidak, skill teratas.
  const selectedItem = items.find((i) => i.skill_id === selected) ?? items[0];
  const d = demand.data;

  return (
    <main className="flex flex-col gap-8 p-6 md:px-12 md:py-10">
      <PageHeader
        title="Skill yang paling diminta"
        meta={d ? <SampleNote total={d.total_jobs} snapshot={d.snapshot_date} roleName={d.role.name} /> : "Memuat data pasar…"}
        actions={
          <>
            <div role="group" aria-label="Role" className="flex rounded-control border border-line-strong bg-surface p-0.5">
              {(roles.data ?? []).map((r) => (
                <button
                  key={r.slug}
                  type="button"
                  aria-pressed={role === r.slug}
                  onClick={() => setRole(r.slug)}
                  className={
                    "h-8 rounded-chip px-3 text-[13px] font-medium " +
                    (role === r.slug ? "bg-ink text-white" : "text-ink-2 hover:bg-sunken")
                  }
                >
                  {r.name}
                </button>
              ))}
            </div>
            <label htmlFor="level" className="sr-only">
              Level
            </label>
            <Select id="level" value={level} onChange={(e) => setLevel(e.target.value)} className="!h-[38px] !w-auto !text-[13px]">
              {LEVELS.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.label}
                </option>
              ))}
            </Select>
          </>
        }
      />

      {demand.error ? (
        <ErrorState error={demand.error} onRetry={demand.reload} />
      ) : demand.loading && !d ? (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
          <Skeleton className="h-[640px]" />
          <Skeleton className="h-[640px]" />
        </div>
      ) : d && d.total_jobs === 0 ? (
        <EmptyState title="Belum ada data lowongan untuk filter ini">
          Statistik muncul setelah lowongan {d.role.name}
          {level ? ` level ${level}` : ""} dikumpulkan dan diekstrak. Jalankan{" "}
          <code className="font-mono text-[13px]">ingest_cli.py send</code> setelah mengisi <code className="font-mono text-[13px]">data/raw_jobs/</code>.
        </EmptyState>
      ) : d ? (
        <div className="flex flex-col gap-4">
          {d.small_sample && <SmallSampleWarning total={d.total_jobs} />}
          <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
            <Panel className="flex flex-col gap-4 p-6">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h2 className="m-0 text-[16px] font-semibold">{items.length} skill teratas</h2>
                <div className="flex gap-4 text-[12px] text-ink-2">
                  <span className="flex items-center gap-1.5">
                    <span className="size-2.5 rounded-[2px] bg-compass" />
                    Sudah kamu miliki
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span className="size-2.5 rounded-[2px] bg-ink" />
                    Belum
                  </span>
                </div>
              </div>
              <div className={"flex flex-col " + (demand.loading ? "opacity-60" : "")}>
                {items.map((it, i) => (
                  <DemandRow
                    key={it.skill_id}
                    rank={i + 1}
                    name={it.name}
                    pct={it.demand_pct}
                    owned={owned.has(it.skill_id)}
                    selected={it.skill_id === selectedItem?.skill_id}
                    onClick={() => setSelected(it.skill_id)}
                  />
                ))}
              </div>
              {!profile.data && (
                <div className="border-t border-line pt-4 text-[13px] text-muted">
                  <Link href="/">Isi profil</Link> agar skill yang sudah kamu kuasai ditandai hijau.
                </div>
              )}
            </Panel>

            {selectedItem && (
              <EvidencePanel
                key={`${selectedItem.skill_id}-${role}-${level}`}
                skillId={selectedItem.skill_id}
                name={selectedItem.name}
                jobCount={selectedItem.job_count}
                requiredCount={selectedItem.required_count}
                total={d.total_jobs}
                role={role}
                level={level}
              />
            )}
          </div>
        </div>
      ) : null}
    </main>
  );
}

function EvidencePanel({
  skillId,
  name,
  jobCount,
  requiredCount,
  total,
  role,
  level,
}: {
  skillId: number;
  name: string;
  jobCount: number;
  requiredCount: number;
  total: number;
  role: string;
  level: string;
}) {
  const [extra, setExtra] = useState<EvidenceJob[]>([]);
  const [more, setMore] = useState<{ loading: boolean; error?: unknown }>({ loading: false });
  const first = useAsync(() => api.evidenceJobs(skillId, { role, level, limit: PAGE }), []);

  const jobs = [...(first.data?.jobs ?? []), ...extra];
  const totalJobs = first.data?.total ?? jobCount;

  async function loadMore() {
    setMore({ loading: true });
    try {
      const next = await api.evidenceJobs(skillId, { role, level, limit: PAGE, offset: jobs.length });
      setExtra((j) => [...j, ...next.jobs]);
      setMore({ loading: false });
    } catch (e) {
      setMore({ loading: false, error: e });
    }
  }

  return (
    <Panel as="aside" className="flex flex-col xl:sticky xl:top-6">
      <div className="flex flex-col gap-2 border-b border-line px-6 pt-6 pb-4">
        <div className="eyebrow">Bukti</div>
        <div className="text-[18px] leading-[26px] font-semibold">{name}</div>
        <div className="text-[13px] text-ink-2">
          Disebut di <span className="font-mono">{jobCount}</span> dari <span className="font-mono">{total}</span> lowongan · wajib
          di <span className="font-mono">{requiredCount}</span>
        </div>
      </div>

      {first.error ? (
        <div className="p-6">
          <ErrorState error={first.error} onRetry={first.reload} title="Daftar lowongan gagal dimuat" />
        </div>
      ) : first.loading ? (
        <div className="flex flex-col gap-3 p-6">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-12" />
          ))}
        </div>
      ) : (
        <ul className="m-0 max-h-[560px] list-none overflow-y-auto p-0">
          {jobs.map((j) => (
            <li key={j.id} className="border-b border-line">
              <JobRow job={j} />
            </li>
          ))}
        </ul>
      )}

      {!first.loading && !first.error && (
        <div className="flex items-center justify-between gap-3 px-6 py-4">
          <span className="font-mono text-[12px] text-muted">
            {jobs.length} / {totalJobs}
          </span>
          {jobs.length < totalJobs && (
            <Button size="sm" onClick={loadMore} loading={more.loading}>
              Muat lebih banyak
            </Button>
          )}
          {more.error !== undefined && <span className="text-[12px] text-error">Gagal memuat.</span>}
        </div>
      )}
    </Panel>
  );
}

function JobRow({ job }: { job: EvidenceJob }) {
  const content = (
    <>
      <span className="flex justify-between gap-3">
        <span className="text-[14px] font-medium">{job.title}</span>
        <span className={"shrink-0 text-[12px] font-medium " + (job.requirement_type === "required" ? "text-ink" : "text-muted")}>
          {job.requirement_type === "required" ? "Wajib" : "Opsional"}
        </span>
      </span>
      <span className="text-[13px] text-muted">
        {[job.company, job.location, job.source_name].filter(Boolean).join(" · ")} ·{" "}
        <span className="font-mono text-[12px]">{formatDate(job.posted_date ?? job.collected_at)}</span>
      </span>
    </>
  );
  const cls = "flex flex-col gap-1 px-6 py-3.5 text-ink no-underline";
  return job.source_url ? (
    <a href={job.source_url} target="_blank" rel="noopener noreferrer" className={cls + " hover:bg-sunken hover:text-ink"}>
      {content}
    </a>
  ) : (
    <div className={cls}>{content}</div>
  );
}
