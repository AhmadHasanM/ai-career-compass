"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { CATEGORY_LABEL } from "@/components/skill-picker";
import {
  Button,
  Chip,
  EmptyState,
  ErrorState,
  PageHeader,
  Panel,
  Skeleton,
  SmallSampleWarning,
} from "@/components/ui";
import { api, ApiError, formatDate, formatPct } from "@/lib/api";
import { useAsync } from "@/lib/use-async";

export default function GapPage() {
  const router = useRouter();
  const gap = useAsync(() => api.gap(), []);
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<unknown>();

  async function buildRoadmap() {
    setGenerating(true);
    setGenError(undefined);
    try {
      await api.generateRoadmap();
      router.push("/roadmap");
    } catch (e) {
      setGenError(e);
      setGenerating(false);
    }
  }

  const needsProfile = gap.error instanceof ApiError && gap.error.code === "profile_required";
  const g = gap.data;

  return (
    <main className="flex flex-col gap-8 p-6 md:px-12 md:py-10">
      <PageHeader
        title="Skill gap"
        meta={
          g ? (
            <>
              Skill yang diminta di ≥ <span className="font-mono">{g.threshold_pct}%</span> lowongan {g.role.name} ·{" "}
              <span className="font-mono">n = {g.total_jobs}</span> · snapshot {formatDate(g.snapshot_date)}
            </>
          ) : undefined
        }
        actions={
          g && g.gaps.length > 0 ? (
            <Button variant="primary" onClick={buildRoadmap} loading={generating}>
              {generating ? "Menyusun roadmap…" : "Buat roadmap"}
            </Button>
          ) : undefined
        }
      />

      {genError !== undefined && <ErrorState error={genError} title="Roadmap gagal dibuat" onRetry={buildRoadmap} />}

      {needsProfile ? (
        <EmptyState title="Isi profil dulu" action={<Link href="/">Buka profil →</Link>}>
          Gap dihitung dari skill yang sudah kamu kuasai dibanding permintaan lowongan untuk target role-mu.
        </EmptyState>
      ) : gap.error ? (
        <ErrorState error={gap.error} onRetry={gap.reload} />
      ) : !g ? (
        <div className="flex flex-col gap-6">
          <Skeleton className="h-28" />
          <Skeleton className="h-[480px]" />
        </div>
      ) : g.total_jobs === 0 ? (
        <EmptyState title="Belum ada data lowongan">
          Gap belum bisa dihitung karena belum ada lowongan {g.role.name} yang selesai diekstrak.
        </EmptyState>
      ) : (
        <>
          {g.small_sample && <SmallSampleWarning total={g.total_jobs} />}

          <Panel className="flex flex-col gap-3 p-6">
            <div className="flex flex-wrap items-baseline justify-between gap-3">
              <h2 className="m-0 text-[16px] font-semibold">Cakupan permintaan pasar</h2>
              <span className="font-mono text-[24px] leading-8">{formatPct(g.coverage_pct)}</span>
            </div>
            <div className="flex h-2 rounded-[2px] bg-sunken" aria-hidden>
              <span className="rounded-[2px] bg-compass" style={{ width: `${g.coverage_pct}%` }} />
            </div>
            <p className="m-0 text-[13px] leading-5 text-muted">
              Porsi total permintaan (skill di atas ambang {g.threshold_pct}%) yang sudah kamu kuasai.{" "}
              <span className="font-mono">{g.owned.length}</span> skill dimiliki, <span className="font-mono">{g.gaps.length}</span> gap.
            </p>
          </Panel>

          <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
            <Panel className="flex flex-col">
              <div className="flex flex-col gap-1 border-b border-line px-6 py-5">
                <h2 className="m-0 text-[16px] font-semibold">Gap prioritas</h2>
                <p className="m-0 text-[13px] text-muted">Skor prioritas = persentase permintaan × bobot kategori.</p>
              </div>
              {g.gaps.length === 0 ? (
                <div className="p-6 text-[14px] text-ink-2">Semua skill di atas ambang sudah kamu kuasai.</div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full border-collapse text-left text-[14px]">
                    <thead>
                      <tr className="border-b border-line text-[12px] text-muted">
                        <th scope="col" className="px-6 py-3 font-medium">#</th>
                        <th scope="col" className="px-2 py-3 font-medium">Skill</th>
                        <th scope="col" className="px-2 py-3 font-medium">Kategori</th>
                        <th scope="col" className="px-2 py-3 text-right font-medium">Diminta</th>
                        <th scope="col" className="px-2 py-3 text-right font-medium">Wajib</th>
                        <th scope="col" className="px-6 py-3 text-right font-medium">Skor</th>
                      </tr>
                    </thead>
                    <tbody>
                      {g.gaps.map((it, i) => (
                        <tr key={it.skill_id} className="border-b border-line last:border-0">
                          <td className="px-6 py-3 font-mono text-[12px] text-muted">{String(i + 1).padStart(2, "0")}</td>
                          <td className="px-2 py-3">
                            <Chip kind="gap">{it.name}</Chip>
                          </td>
                          <td className="px-2 py-3 text-[13px] text-ink-2">{CATEGORY_LABEL[it.category] ?? it.category}</td>
                          <td className="px-2 py-3 text-right font-mono text-[13px]">{formatPct(it.demand_pct)}</td>
                          <td className="px-2 py-3 text-right font-mono text-[13px] text-ink-2">{formatPct(it.required_pct)}</td>
                          <td className="px-6 py-3 text-right font-mono text-[13px] font-medium">{it.priority_score.toFixed(1)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </Panel>

            <Panel as="aside" className="flex flex-col gap-4 p-6">
              <h2 className="m-0 text-[16px] font-semibold">Sudah kamu kuasai</h2>
              {g.owned.length === 0 ? (
                <p className="m-0 text-[14px] text-ink-2">
                  Belum ada skill di atas ambang yang kamu kuasai. <Link href="/">Perbarui profil</Link> jika ada yang terlewat.
                </p>
              ) : (
                <ul className="m-0 flex list-none flex-col gap-2 p-0">
                  {g.owned.map((o) => (
                    <li key={o.skill_id} className="flex items-center justify-between gap-3">
                      <Chip kind="owned">{o.name}</Chip>
                      <span className="font-mono text-[13px] text-ink-2">{formatPct(o.demand_pct)}</span>
                    </li>
                  ))}
                </ul>
              )}
              <p className="m-0 border-t border-line pt-4 text-[13px] text-muted">
                Angka di kanan: persentase lowongan yang menyebut skill. Lihat buktinya di{" "}
                <Link href="/dashboard">Market insight</Link>.
              </p>
            </Panel>
          </div>
        </>
      )}
    </main>
  );
}
