"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { SkillPicker, type SelectedSkill } from "@/components/skill-picker";
import { Button, ErrorState, Field, Input, PageHeader, Panel, Select, Skeleton } from "@/components/ui";
import { api, ApiError, type Profile, type Role, type Skill } from "@/lib/api";
import { useAsync } from "@/lib/use-async";

type FormState = {
  education: string;
  current_job: string;
  target_role_id: string;
  hours_per_week: string;
  skills: SelectedSkill[];
};

function fromProfile(p: Profile | null, roles: Role[]): FormState {
  return {
    education: p?.education ?? "",
    current_job: p?.current_job ?? "",
    target_role_id: String(p?.target_role?.id ?? roles[0]?.id ?? ""),
    hours_per_week: p?.hours_per_week ? String(p.hours_per_week) : "",
    skills: p?.skills.map((s) => ({ skill_id: s.skill_id, proficiency: s.proficiency })) ?? [],
  };
}

export default function ProfilePage() {
  const router = useRouter();
  const initial = useAsync(() => Promise.all([api.roles(), api.skills(), api.profile()]), []);
  const [form, setForm] = useState<FormState | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [status, setStatus] = useState<"idle" | "saving" | "generating">("idle");
  const [submitError, setSubmitError] = useState<{ error: unknown; phase: "saving" | "generating" }>();
  const [savedAt, setSavedAt] = useState<string>();

  // Sebelum pengguna mengubah apa pun, form berisi profil yang tersimpan.
  const current = form ?? (initial.data ? fromProfile(initial.data[2], initial.data[0]) : null);

  if (initial.error) return <main className="p-6 md:px-12 md:py-10"><ErrorState error={initial.error} onRetry={initial.reload} /></main>;

  const [roles, skills, profile]: [Role[], Skill[], Profile | null] = initial.data ?? [[], [], null];
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) =>
    setForm((f) => {
      const base = f ?? current;
      return base ? { ...base, [k]: v } : base;
    });

  async function submit(e: FormEvent, generate: boolean) {
    e.preventDefault();
    const form = current;
    if (!form) return;
    setFieldErrors({});
    setSubmitError(undefined);
    setStatus("saving");
    let phase: "saving" | "generating" = "saving";
    try {
      await api.saveProfile({
        education: form.education || null,
        current_job: form.current_job || null,
        target_role_id: form.target_role_id ? Number(form.target_role_id) : null,
        hours_per_week: form.hours_per_week ? Number(form.hours_per_week) : null,
        skills: form.skills,
      });
      if (!generate) {
        setSavedAt(new Date().toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" }));
        setStatus("idle");
        return;
      }
      phase = "generating";
      setStatus("generating");
      await api.generateRoadmap();
      router.push("/roadmap");
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFieldErrors(err.fields);
      setSubmitError({ error: err, phase });
      setStatus("idle");
    }
  }

  const busy = status !== "idle";

  return (
    <main className="flex flex-col gap-8 p-6 md:px-12 md:py-10">
      <PageHeader
        title={profile ? "Profil & skill" : "Mulai dari profilmu"}
        meta="Dipakai untuk menghitung gap skill terhadap lowongan dan menyusun urutan belajar. Tanpa login: tersimpan di sesi browser ini."
      />

      {!current ? (
        <div className="grid gap-6 lg:grid-cols-[360px_minmax(0,1fr)]">
          <Skeleton className="h-[360px]" />
          <Skeleton className="h-[480px]" />
        </div>
      ) : (
        <form onSubmit={(e) => submit(e, true)} className="flex flex-col gap-6" noValidate>
          <div className="grid items-start gap-6 lg:grid-cols-[360px_minmax(0,1fr)]">
            <Panel className="flex flex-col gap-5 p-6">
              <h2 className="m-0 text-[16px] font-semibold">Latar belakang</h2>
              <Field id="education" label="Pendidikan terakhir" error={fieldErrors.education}>
                <Input
                  id="education"
                  placeholder="Contoh: S1 Teknik Informatika"
                  value={current.education}
                  invalid={!!fieldErrors.education}
                  onChange={(e) => set("education", e.target.value)}
                />
              </Field>
              <Field id="current_job" label="Pekerjaan saat ini" error={fieldErrors.current_job}>
                <Input
                  id="current_job"
                  placeholder="Contoh: Backend Developer"
                  value={current.current_job}
                  invalid={!!fieldErrors.current_job}
                  onChange={(e) => set("current_job", e.target.value)}
                />
              </Field>
              <Field id="target_role" label="Target role" error={fieldErrors.target_role_id}>
                <Select id="target_role" value={current.target_role_id} onChange={(e) => set("target_role_id", e.target.value)}>
                  {roles.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field
                id="hours"
                label="Jam belajar per minggu"
                hint="Dipakai untuk estimasi durasi tiap node roadmap."
                error={fieldErrors.hours_per_week}
              >
                <Input
                  id="hours"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={80}
                  placeholder="10"
                  value={current.hours_per_week}
                  invalid={!!fieldErrors.hours_per_week}
                  onChange={(e) => set("hours_per_week", e.target.value)}
                />
              </Field>
            </Panel>

            <Panel className="flex flex-col gap-4 p-6">
              <h2 className="m-0 text-[16px] font-semibold">Skill</h2>
              {fieldErrors.skills && <div className="text-[13px] text-error">{fieldErrors.skills}</div>}
              <SkillPicker skills={skills} value={current.skills} onChange={(v) => set("skills", v)} />
            </Panel>
          </div>

          {submitError && !(submitError.error instanceof ApiError && submitError.error.fields) && (
            <ErrorState
              error={submitError.error}
              title={submitError.phase === "saving" ? "Profil gagal disimpan" : "Profil tersimpan, tapi roadmap gagal dibuat"}
            />
          )}

          <div className="flex flex-wrap items-center gap-3 border-t border-line pt-6">
            <Button type="submit" variant="primary" loading={busy && status === "generating"} disabled={busy}>
              {status === "generating" ? "Menyusun roadmap…" : "Simpan dan buat roadmap"}
            </Button>
            <Button type="button" onClick={(e) => submit(e, false)} loading={status === "saving"} disabled={busy}>
              Simpan saja
            </Button>
            <span className="text-[13px] text-muted" role="status" aria-live="polite">
              {status === "generating"
                ? "Mengurutkan skill dan meminta penjelasan tiap langkah (maks. ±20 detik)…"
                : savedAt
                  ? `Tersimpan ${savedAt}.`
                  : ""}
            </span>
          </div>
        </form>
      )}
    </main>
  );
}
