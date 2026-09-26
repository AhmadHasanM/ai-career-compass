// Komponen dasar Evidence Ledger (DESIGN.md).
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";
import { ApiError, formatDate, type NodeStatus } from "@/lib/api";

function cx(...classes: (string | false | null | undefined)[]) {
  return classes.filter(Boolean).join(" ");
}

type ButtonVariant = "primary" | "secondary" | "ghost";

const buttonStyles: Record<ButtonVariant, string> = {
  primary: "bg-compass border-compass text-white hover:bg-compass-strong hover:border-compass-strong",
  secondary: "bg-surface border-line-strong text-ink hover:bg-sunken",
  ghost: "bg-transparent border-transparent text-ink-2 hover:bg-sunken",
};

export function Button({
  variant = "secondary",
  size = "md",
  loading,
  className,
  children,
  disabled,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; size?: "sm" | "md"; loading?: boolean }) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cx(
        "inline-flex items-center justify-center gap-2 rounded-control border font-medium whitespace-nowrap",
        "transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        size === "md" ? "h-10 px-4 text-[14px]" : "h-8 px-3 text-[13px]",
        buttonStyles[variant],
        className,
      )}
    >
      {loading && <Spinner />}
      {children}
    </button>
  );
}

export function Spinner({ className }: { className?: string }) {
  return (
    <svg className={cx("size-4 animate-spin motion-reduce:animate-none", className)} viewBox="0 0 16 16" fill="none" aria-hidden>
      <circle cx="8" cy="8" r="6" stroke="currentColor" strokeOpacity="0.3" strokeWidth="2" />
      <path d="M14 8a6 6 0 0 0-6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

const controlBase =
  "h-10 w-full rounded-control border border-line-strong bg-surface px-3 text-[15px] text-ink " +
  "focus:border-compass focus:outline-none focus:ring-[3px] focus:ring-compass-soft";

export function Field({
  id,
  label,
  hint,
  error,
  children,
}: {
  id: string;
  label: string;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[13px] font-medium text-ink-2">
        {label}
      </label>
      {children}
      {error ? (
        <span id={`${id}-error`} className="text-[12px] text-error">
          {error}
        </span>
      ) : (
        hint && <span className="text-[12px] text-muted">{hint}</span>
      )}
    </div>
  );
}

export function Input({ className, invalid, ...rest }: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return (
    <input
      {...rest}
      aria-invalid={invalid || undefined}
      aria-describedby={invalid && rest.id ? `${rest.id}-error` : undefined}
      className={cx(controlBase, invalid && "border-error", className)}
    />
  );
}

export function Select({ className, children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select {...rest} className={cx(controlBase, "pr-8", className)}>
      {children}
    </select>
  );
}

export function CheckIcon({ className }: { className?: string }) {
  return (
    <svg className={className} width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden>
      <path d="M2.5 6.2l2.2 2.2 4.8-4.8" />
    </svg>
  );
}

// Dimiliki = hijau + centang; gap = amber + kata "gap"; netral = border. Tidak pernah warna saja.
export function Chip({ kind = "neutral", children }: { kind?: "owned" | "gap" | "neutral"; children: ReactNode }) {
  const styles = {
    owned: "bg-compass-soft text-compass-strong",
    gap: "bg-gap-soft text-gap-strong",
    neutral: "border border-line-strong text-ink-2",
  }[kind];
  return (
    <span className={cx("inline-flex h-7 items-center gap-1.5 rounded-chip px-2.5 text-[13px] font-medium", styles)}>
      {kind === "owned" && <CheckIcon />}
      {children}
      {kind === "gap" && <span className="font-normal">· gap</span>}
    </span>
  );
}

export function Panel({ className, children, as: Tag = "section" }: { className?: string; children: ReactNode; as?: "section" | "aside" | "div" }) {
  return <Tag className={cx("rounded-panel border border-line bg-surface", className)}>{children}</Tag>;
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cx("skeleton", className)} aria-hidden />;
}

export function PageHeader({ title, meta, actions }: { title: string; meta?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-6">
      <div className="flex flex-col gap-1.5">
        <h1 className="m-0 text-[24px] leading-8 font-semibold">{title}</h1>
        {meta && <div className="text-[13px] leading-5 text-muted">{meta}</div>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-3">{actions}</div>}
    </div>
  );
}

// "n = X lowongan · snapshot DD Mmm YYYY" — setiap angka punya sumber (DESIGN.md prinsip 1).
export function SampleNote({ total, snapshot, roleName }: { total: number; snapshot: string | null; roleName?: string }) {
  return (
    <>
      Persentase lowongan yang menyebut skill · <span className="font-mono">n = {total}</span> lowongan
      {roleName ? ` ${roleName}` : ""} · snapshot {formatDate(snapshot)}
    </>
  );
}

export function SmallSampleWarning({ total }: { total: number }) {
  return (
    <div role="note" className="rounded-control bg-gap-soft px-4 py-3 text-[13px] leading-5 text-gap-strong">
      Sampel masih kecil (<span className="font-mono">n = {total}</span>, kurang dari 30 lowongan). Baca persentase
      sebagai gambaran awal, bukan angka pasti.
    </div>
  );
}

export function ErrorState({ error, onRetry, title = "Data gagal dimuat" }: { error: unknown; onRetry?: () => void; title?: string }) {
  const message = error instanceof ApiError ? error.message : "Terjadi kesalahan yang tidak terduga.";
  return (
    <div role="alert" className="flex flex-col items-start gap-3 rounded-panel border border-line bg-surface p-6">
      <div className="text-[15px] font-semibold text-error">{title}</div>
      <p className="m-0 text-[14px] leading-6 text-ink-2">{message}</p>
      {onRetry && (
        <Button size="sm" onClick={onRetry}>
          Coba lagi
        </Button>
      )}
    </div>
  );
}

export function EmptyState({ title, children, action }: { title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-start gap-3 rounded-panel border border-dashed border-line-strong p-8">
      <div className="text-[16px] font-semibold">{title}</div>
      {children && <div className="max-w-xl text-[14px] leading-6 text-ink-2">{children}</div>}
      {action}
    </div>
  );
}

// Baris demand: rank · nama · bar (hijau jika dimiliki) · persen. Baris adalah tombol.
export function DemandRow({
  rank,
  name,
  pct,
  owned,
  selected,
  onClick,
}: {
  rank: number;
  name: string;
  pct: number;
  owned: boolean;
  selected?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={cx(
        "grid h-10 w-full grid-cols-[24px_minmax(0,160px)_minmax(0,1fr)_56px] items-center gap-3 rounded-chip px-2 text-left text-[14px]",
        "cursor-pointer hover:bg-sunken",
        selected && "bg-sunken",
      )}
    >
      <span className="font-mono text-[12px] text-muted">{String(rank).padStart(2, "0")}</span>
      <span className="flex items-center gap-1.5 truncate">
        {owned && <CheckIcon className="shrink-0 text-compass" />}
        <span className="truncate">{name}</span>
      </span>
      <span className="flex h-2 rounded-[2px] bg-sunken" aria-hidden>
        <span className={cx("rounded-[2px]", owned ? "bg-compass" : "bg-ink")} style={{ width: `${Math.min(pct, 100)}%` }} />
      </span>
      <span className="text-right font-mono text-[13px]">{pct % 1 ? pct.toFixed(1) : pct}%</span>
    </button>
  );
}

export function CitationMarker({ n, active, onClick }: { n: number; active?: boolean; onClick?: () => void }) {
  return (
    <a
      href={`#src-${n}`}
      onClick={(e) => {
        e.preventDefault();
        onClick?.();
      }}
      aria-label={`Sumber ${n}`}
      className={cx(
        "mx-0.5 inline-block rounded-chip border px-[5px] align-[1px] font-mono text-[12px] leading-[18px] text-compass-strong no-underline",
        active ? "border-compass bg-compass-soft" : "border-line-strong",
      )}
    >
      {n}
    </a>
  );
}

const statusMeta: Record<NodeStatus, { label: string; cls: string; dot: string }> = {
  todo: { label: "Belum", cls: "border border-line-strong text-ink-2", dot: "border-[1.5px] border-muted" },
  in_progress: {
    label: "Berjalan",
    cls: "bg-sunken text-ink",
    dot: "border-[1.5px] border-ink bg-[linear-gradient(90deg,#16181D_50%,transparent_50%)]",
  },
  done: { label: "Selesai", cls: "bg-compass-soft text-compass-strong", dot: "bg-compass" },
};

// Bentuk ikon (kosong, setengah, penuh) membawa arti yang sama dengan warnanya.
export function StatusBadge({ status }: { status: NodeStatus }) {
  const m = statusMeta[status];
  return (
    <span className={cx("inline-flex h-6 items-center gap-1.5 rounded-chip px-2 text-[12px] font-medium", m.cls)}>
      <span className={cx("box-border size-2 rounded-full", m.dot)} />
      {m.label}
    </span>
  );
}
