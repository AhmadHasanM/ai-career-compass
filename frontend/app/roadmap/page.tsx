"use client";

import Link from "next/link";
import { useCallback, useState } from "react";
import { RoadmapGraph } from "@/components/roadmap-graph";
import { Button, EmptyState, ErrorState, PageHeader, Panel, Skeleton, StatusBadge } from "@/components/ui";
import { api, ApiError, formatDate, type LearningResource, type Roadmap, type RoadmapNode } from "@/lib/api";
import { useAsync } from "@/lib/use-async";

const TYPE_LABEL: Record<string, string> = {
  course: "Kursus",
  docs: "Dokumentasi",
  video: "Video",
  article: "Artikel",
  book: "Buku",
  tutorial: "Tutorial",
};
const LEVEL_LABEL: Record<string, string> = { beginner: "pemula", intermediate: "menengah", advanced: "lanjutan" };

export default function RoadmapPage() {
  const loaded = useAsync(() => api.roadmap(), []);
  const [generated, setGenerated] = useState<Roadmap | null>(null);
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<unknown>();
  const [selected, setSelected] = useState<number | null>(null);

  const roadmap = generated ?? loaded.data ?? null;
  const onSelect = useCallback((id: number) => setSelected(id), []);

  async function generate() {
    setGenerating(true);
    setGenError(undefined);
    try {
      setGenerated(await api.generateRoadmap());
      setSelected(null);
    } catch (e) {
      setGenError(e);
    } finally {
      setGenerating(false);
    }
  }

  const needsProfile =
    (loaded.error instanceof ApiError && loaded.error.code === "profile_required") ||
    (genError instanceof ApiError && genError.code === "profile_required");
  const current = roadmap?.nodes.find((n) => n.skill_id === selected) ?? roadmap?.nodes[0];

  return (
    <main className="flex flex-col gap-8 p-6 md:px-12 md:py-10">
      <PageHeader
        title={roadmap ? `Roadmap ${roadmap.role.name}` : "Roadmap"}
        meta={
          roadmap ? (
            <>
              <span className="font-mono">{roadmap.nodes.length}</span> langkah · ±
              <span className="font-mono">{roadmap.total_weeks}</span> minggu dengan{" "}
              <span className="font-mono">{roadmap.hours_per_week ?? "-"}</span> jam/minggu · versi{" "}
              <span className="font-mono">v{roadmap.version}</span> · {formatDate(roadmap.generated_at)}
              {roadmap.model_name === "fallback" && " · penjelasan otomatis (LLM tidak tersedia)"}
            </>
          ) : undefined
        }
        actions={
          roadmap && (
            <Button onClick={generate} loading={generating}>
              {generating ? "Menyusun ulang…" : "Susun ulang roadmap"}
            </Button>
          )
        }
      />

      {genError !== undefined && !needsProfile && <ErrorState error={genError} title="Roadmap gagal dibuat" onRetry={generate} />}

      {needsProfile ? (
        <EmptyState title="Isi profil dulu" action={<Link href="/">Buka profil →</Link>}>
          Roadmap disusun dari skill yang belum kamu kuasai dibanding permintaan lowongan untuk target role-mu.
        </EmptyState>
      ) : loaded.error ? (
        <ErrorState error={loaded.error} onRetry={loaded.reload} />
      ) : loaded.loading && !roadmap ? (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <Skeleton className="h-[480px]" />
          <Skeleton className="h-[480px]" />
        </div>
      ) : !roadmap ? (
        <EmptyState
          title="Belum ada roadmap"
          action={
            <Button variant="primary" onClick={generate} loading={generating}>
              {generating ? "Menyusun roadmap…" : "Buat roadmap"}
            </Button>
          }
        >
          Skill diurutkan berdasarkan prasyarat dan seberapa sering diminta lowongan. Tiap langkah dilengkapi alasan,
          estimasi durasi, dan sumber belajar.
        </EmptyState>
      ) : roadmap.nodes.length === 0 ? (
        <EmptyState title="Tidak ada gap yang perlu dikejar">
          Semua skill yang diminta di atas ambang sudah ada di profilmu, atau data lowongan belum cukup.{" "}
          <Link href="/gap">Lihat skill gap</Link>.
        </EmptyState>
      ) : (
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
          <div className="flex min-w-0 flex-col gap-6">
            <Panel className="overflow-hidden">
              <RoadmapGraph roadmap={roadmap} selected={current?.skill_id ?? null} onSelect={onSelect} />
            </Panel>
            <StepList roadmap={roadmap} selected={current?.skill_id} onSelect={onSelect} />
          </div>
          {current && <NodeDetail node={current} roadmap={roadmap} onSelect={onSelect} />}
        </div>
      )}
    </main>
  );
}

// Daftar langkah berurutan: alternatif graf yang bisa dipakai dengan keyboard dan pembaca layar.
function StepList({ roadmap, selected, onSelect }: { roadmap: Roadmap; selected?: number; onSelect: (id: number) => void }) {
  return (
    <Panel className="flex flex-col">
      <h2 className="m-0 border-b border-line px-6 py-4 text-[16px] font-semibold">Urutan belajar</h2>
      <ol className="m-0 list-none p-0">
        {roadmap.nodes.map((n) => (
          <li key={n.id} className="border-b border-line last:border-0">
            <button
              type="button"
              onClick={() => onSelect(n.skill_id)}
              aria-current={n.skill_id === selected ? "step" : undefined}
              className={
                "grid w-full grid-cols-[32px_minmax(0,1fr)_auto] items-center gap-3 px-6 py-3 text-left hover:bg-sunken " +
                (n.skill_id === selected ? "bg-sunken" : "")
              }
            >
              <span className="font-mono text-[12px] text-muted">{String(n.order_index + 1).padStart(2, "0")}</span>
              <span className="flex min-w-0 flex-col">
                <span className="truncate text-[14px] font-medium">{n.name}</span>
                <span className="text-[12px] text-muted">{n.stage}</span>
              </span>
              <span className="flex gap-4 font-mono text-[12px] text-ink-2">
                <span>{n.demand_pct != null ? `${n.demand_pct}%` : "prasyarat"}</span>
                {n.est_weeks != null && <span>{n.est_weeks} mgg</span>}
              </span>
            </button>
          </li>
        ))}
      </ol>
    </Panel>
  );
}

function NodeDetail({ node, roadmap, onSelect }: { node: RoadmapNode; roadmap: Roadmap; onSelect: (id: number) => void }) {
  const byId = new Map(roadmap.nodes.map((n) => [n.skill_id, n]));
  const prereqs = roadmap.edges.filter((e) => e.to_skill_id === node.skill_id).map((e) => byId.get(e.from_skill_id)!);
  const unlocks = roadmap.edges.filter((e) => e.from_skill_id === node.skill_id).map((e) => byId.get(e.to_skill_id)!);

  return (
    <Panel as="aside" className="flex flex-col xl:sticky xl:top-6">
      <div className="flex flex-col gap-3 border-b border-line p-6">
        <div className="flex items-center justify-between gap-3">
          <span className="eyebrow">
            {node.stage} · langkah {node.order_index + 1}
          </span>
          <StatusBadge status={node.status} />
        </div>
        <h2 className="m-0 text-[18px] leading-[26px] font-semibold">{node.name}</h2>
        <dl className="m-0 grid grid-cols-2 gap-3">
          <div>
            <dt className="text-[12px] text-muted">Permintaan</dt>
            <dd className="m-0 font-mono text-[14px]">{node.demand_pct != null ? `${node.demand_pct}% lowongan` : "prasyarat"}</dd>
          </div>
          <div>
            <dt className="text-[12px] text-muted">Estimasi</dt>
            <dd className="m-0 font-mono text-[14px]">{node.est_weeks != null ? `${node.est_weeks} minggu` : "-"}</dd>
          </div>
        </dl>
        {node.rationale && <p className="m-0 text-[14px] leading-[22px] text-ink-2">{node.rationale}</p>}
      </div>

      {(prereqs.length > 0 || unlocks.length > 0) && (
        <div className="flex flex-col gap-2 border-b border-line px-6 py-4 text-[13px]">
          {prereqs.length > 0 && <Relation label="Butuh" nodes={prereqs} onSelect={onSelect} />}
          {unlocks.length > 0 && <Relation label="Membuka" nodes={unlocks} onSelect={onSelect} />}
        </div>
      )}

      <div className="flex flex-col gap-3 p-6">
        <div className="eyebrow">Sumber belajar</div>
        {node.resources.length === 0 ? (
          <p className="m-0 text-[13px] text-muted">Belum ada sumber belajar terkurasi untuk skill ini.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-3 p-0">
            {node.resources.map((r) => (
              <ResourceItem key={r.id} r={r} />
            ))}
          </ul>
        )}
      </div>
    </Panel>
  );
}

function Relation({ label, nodes, onSelect }: { label: string; nodes: RoadmapNode[]; onSelect: (id: number) => void }) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="w-16 text-muted">{label}</span>
      {nodes.map((n) => (
        <button
          key={n.skill_id}
          type="button"
          onClick={() => onSelect(n.skill_id)}
          className="h-6 rounded-chip border border-line-strong px-2 text-[12px] font-medium text-ink-2 hover:border-ink hover:text-ink"
        >
          {n.name}
        </button>
      ))}
    </div>
  );
}

function ResourceItem({ r }: { r: LearningResource }) {
  const meta = [
    TYPE_LABEL[r.type] ?? r.type,
    r.level ? LEVEL_LABEL[r.level] : null,
    r.language === "id" ? "Bahasa Indonesia" : "Inggris",
    r.is_free ? "gratis" : "berbayar",
    r.est_hours ? `±${r.est_hours} jam` : null,
  ].filter(Boolean);
  return (
    <li className="flex flex-col gap-0.5">
      <a href={r.url} target="_blank" rel="noopener noreferrer" className="text-[14px] font-medium">
        {r.title}
      </a>
      <span className="text-[12px] text-muted">{meta.join(" · ")}</span>
    </li>
  );
}
