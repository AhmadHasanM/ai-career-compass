"use client";

import { useMemo, useState } from "react";
import type { Proficiency, Skill } from "@/lib/api";
import { Chip, Input, Select } from "@/components/ui";

export const CATEGORY_LABEL: Record<string, string> = {
  language: "Bahasa pemrograman",
  framework: "Framework & library",
  llm: "LLM",
  ml: "Machine learning",
  mlops: "MLOps & tooling",
  cloud: "Cloud",
  data: "Data",
};
const CATEGORY_ORDER = ["language", "framework", "llm", "ml", "mlops", "cloud", "data"];

const PROFICIENCY_LABEL: Record<Proficiency, string> = {
  beginner: "Pemula",
  intermediate: "Menengah",
  advanced: "Mahir",
};

export type SelectedSkill = { skill_id: number; proficiency: Proficiency };

// Skor kecocokan pencarian: 0 = persis, 1 = awalan, 2 = substring, null = tidak cocok.
// Mencegah "git" memilih CI/CD (alias "github actions") alih-alih Git.
export function matchScore(skill: Skill, q: string): number | null {
  const needle = q.trim().toLowerCase();
  if (!needle) return 2;
  const terms = [skill.name.toLowerCase(), skill.slug, ...skill.aliases];
  if (terms.some((t) => t === needle)) return 0;
  if (terms.some((t) => t.startsWith(needle))) return 1;
  if (terms.some((t) => t.includes(needle))) return 2;
  return null;
}

export function SkillPicker({
  skills,
  value,
  onChange,
}: {
  skills: Skill[];
  value: SelectedSkill[];
  onChange: (v: SelectedSkill[]) => void;
}) {
  const [query, setQuery] = useState("");
  const byId = useMemo(() => new Map(skills.map((s) => [s.id, s])), [skills]);
  const selectedIds = new Set(value.map((v) => v.skill_id));

  const groups = useMemo(() => {
    const q = query.trim();
    const available = skills.filter((s) => !selectedIds.has(s.id));
    if (q) {
      // Saat mencari: satu daftar urut kecocokan, supaya Enter memilih hasil terbaik.
      const ranked = available
        .map((s) => ({ s, score: matchScore(s, q) }))
        .filter((r): r is { s: Skill; score: number } => r.score !== null)
        .sort((a, b) => a.score - b.score || a.s.name.localeCompare(b.s.name))
        .map((r) => r.s);
      return ranked.length ? [{ cat: "hasil", items: ranked }] : [];
    }
    return CATEGORY_ORDER.map((cat) => ({ cat, items: available.filter((s) => s.category === cat) })).filter(
      (g) => g.items.length > 0,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [skills, query, value]);

  const add = (id: number) => {
    onChange([...value, { skill_id: id, proficiency: "beginner" }]);
    setQuery("");
  };
  const remove = (id: number) => onChange(value.filter((v) => v.skill_id !== id));
  const setLevel = (id: number, p: Proficiency) =>
    onChange(value.map((v) => (v.skill_id === id ? { ...v, proficiency: p } : v)));

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <div className="text-[13px] font-medium text-ink-2">
          Skill yang sudah kamu kuasai <span className="font-mono text-muted">({value.length})</span>
        </div>
        {value.length === 0 ? (
          <div className="rounded-control border border-dashed border-line-strong px-4 py-3 text-[14px] text-muted">
            Belum ada skill dipilih. Cari atau pilih dari daftar di bawah.
          </div>
        ) : (
          <ul className="m-0 flex list-none flex-col divide-y divide-line rounded-control border border-line p-0">
            {value.map((v) => {
              const s = byId.get(v.skill_id);
              if (!s) return null;
              return (
                <li key={v.skill_id} className="flex items-center gap-3 px-3 py-2">
                  <Chip kind="owned">{s.name}</Chip>
                  <span className="hidden text-[12px] text-muted sm:inline">{CATEGORY_LABEL[s.category]}</span>
                  <label className="sr-only" htmlFor={`prof-${v.skill_id}`}>
                    Tingkat {s.name}
                  </label>
                  <Select
                    id={`prof-${v.skill_id}`}
                    value={v.proficiency}
                    onChange={(e) => setLevel(v.skill_id, e.target.value as Proficiency)}
                    className="ml-auto !h-8 !w-[128px] !text-[13px]"
                  >
                    {Object.entries(PROFICIENCY_LABEL).map(([k, label]) => (
                      <option key={k} value={k}>
                        {label}
                      </option>
                    ))}
                  </Select>
                  <button
                    type="button"
                    onClick={() => remove(v.skill_id)}
                    aria-label={`Hapus ${s.name}`}
                    className="flex size-8 items-center justify-center rounded-control text-muted hover:bg-sunken hover:text-ink"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" stroke="currentColor" strokeWidth="1.5" aria-hidden>
                      <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" />
                    </svg>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>

      <div className="flex flex-col gap-3">
        <label htmlFor="skill-search" className="text-[13px] font-medium text-ink-2">
          Tambah skill
        </label>
        <Input
          id="skill-search"
          type="search"
          placeholder="Cari: python, postgres, k8s, rag…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              const first = groups[0]?.items[0];
              if (first) add(first.id);
            }
          }}
        />
        <div className="flex max-h-[360px] flex-col gap-4 overflow-y-auto pr-1">
          {groups.length === 0 && <div className="text-[14px] text-muted">Tidak ada skill yang cocok dengan “{query}”.</div>}
          {groups.map((g) => (
            <div key={g.cat} className="flex flex-col gap-2">
              <div className="eyebrow">{g.cat === "hasil" ? "Hasil pencarian · Enter memilih yang pertama" : CATEGORY_LABEL[g.cat]}</div>
              <div className="flex flex-wrap gap-2">
                {g.items.map((s) => (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => add(s.id)}
                    title={s.description ?? undefined}
                    className="inline-flex h-7 items-center gap-1 rounded-chip border border-line-strong bg-surface px-2.5 text-[13px] font-medium text-ink-2 hover:border-compass hover:text-compass-strong"
                  >
                    <span aria-hidden>+</span> {s.name}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
