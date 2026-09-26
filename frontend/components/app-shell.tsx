"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { api, formatDate } from "@/lib/api";
import { useAsync } from "@/lib/use-async";

const NAV = [
  { href: "/", label: "Profil" },
  { href: "/dashboard", label: "Market insight" },
  { href: "/gap", label: "Skill gap" },
  { href: "/roadmap", label: "Roadmap" },
  { href: "/chat", label: "Tanya" },
];

export function CompassLogo() {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden>
      <circle cx="11" cy="11" r="9.25" />
      <path d="M14.5 7.5l-2 5-5 2 2-5z" fill="#1E6B52" stroke="#1E6B52" strokeLinejoin="round" />
    </svg>
  );
}

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  // Tanggal snapshot data untuk sidebar; kegagalan cukup disembunyikan (halaman menampilkan errornya sendiri).
  const market = useAsync(() => api.skillDemand({ limit: 1 }), []);
  const snapshot = market.data?.snapshot_date;

  const links = NAV.map((item) => {
    const active = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
    return (
      <Link
        key={item.href}
        href={item.href}
        aria-current={active ? "page" : undefined}
        className={
          "flex h-9 shrink-0 items-center rounded-control px-3 text-[14px] no-underline " +
          (active ? "bg-sunken font-medium text-ink" : "text-ink-2 hover:bg-sunken hover:text-ink")
        }
      >
        {item.label}
      </Link>
    );
  });

  return (
    <div className="flex min-h-full flex-col md:flex-row">
      <nav
        aria-label="Navigasi utama"
        className="flex shrink-0 flex-col gap-3 border-b border-line px-4 py-4 md:sticky md:top-0 md:h-screen md:w-[232px] md:gap-8 md:border-r md:border-b-0 md:py-6"
      >
        <Link href="/" className="flex items-center gap-2.5 px-2 text-ink no-underline hover:text-ink">
          <CompassLogo />
          <span className="text-[16px] font-semibold tracking-[-0.01em]">Career Compass</span>
        </Link>
        <div className="flex gap-0.5 overflow-x-auto md:flex-col">{links}</div>
        <div className="mt-auto hidden px-2 font-mono text-[12px] leading-[18px] text-muted md:block">
          Data snapshot
          <br />
          {snapshot ? formatDate(snapshot) : "belum ada data"}
        </div>
      </nav>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
