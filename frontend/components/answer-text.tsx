// Render jawaban chatbot: subset markdown (paragraf, daftar, **tebal**, `kode`, [teks](url)) + penanda sitasi [n].
// Sengaja tanpa dangerouslySetInnerHTML: output LLM tidak pernah menjadi HTML.
import type { ReactNode } from "react";
import { CitationMarker } from "@/components/ui";

type Props = { text: string; active?: number | null; onCite?: (n: number) => void };

const INLINE = /(\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\(https?:\/\/[^)\s]+\)|\[\d+(?:\s*,\s*\d+)*\])/g;

function inline(text: string, props: Props, keyBase: string): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let i = 0;
  for (const m of text.matchAll(INLINE)) {
    const tok = m[0];
    if (m.index! > last) out.push(text.slice(last, m.index));
    const key = `${keyBase}-${i++}`;
    if (tok.startsWith("**")) {
      out.push(
        <strong key={key} className="font-semibold">
          {tok.slice(2, -2)}
        </strong>,
      );
    } else if (tok.startsWith("`")) {
      out.push(
        <code key={key} className="rounded-chip bg-sunken px-1 font-mono text-[13px]">
          {tok.slice(1, -1)}
        </code>,
      );
    } else if (tok.includes("](")) {
      const [, label, url] = tok.match(/^\[([^\]]+)\]\((.+)\)$/)!;
      out.push(
        <a key={key} href={url} target="_blank" rel="noopener noreferrer">
          {label}
        </a>,
      );
    } else {
      for (const n of tok.slice(1, -1).split(",").map((s) => Number(s.trim()))) {
        out.push(<CitationMarker key={`${key}-${n}`} n={n} active={props.active === n} onClick={() => props.onCite?.(n)} />);
      }
    }
    last = m.index! + tok.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

export function AnswerText(props: Props) {
  const blocks: ReactNode[] = [];
  const lines = props.text.split("\n");
  let list: { ordered: boolean; items: string[] } | null = null;
  let para: string[] = [];

  const flushPara = () => {
    if (para.length) {
      const k = `p${blocks.length}`;
      blocks.push(
        <p key={k} className="m-0 text-[15px] leading-[26px] [text-wrap:pretty]">
          {inline(para.join(" "), props, k)}
        </p>,
      );
      para = [];
    }
  };
  const flushList = () => {
    if (list) {
      const k = `l${blocks.length}`;
      const Tag = list.ordered ? "ol" : "ul";
      blocks.push(
        <Tag key={k} className={"m-0 flex flex-col gap-1.5 pl-5 text-[15px] leading-[26px] " + (list.ordered ? "list-decimal" : "list-disc")}>
          {list.items.map((it, i) => (
            <li key={i}>{inline(it, props, `${k}-${i}`)}</li>
          ))}
        </Tag>,
      );
      list = null;
    }
  };

  for (const raw of lines) {
    const line = raw.trim();
    const bullet = line.match(/^[*-]\s+(.*)$/);
    const numbered = line.match(/^\d+[.)]\s+(.*)$/);
    if (!line) {
      flushPara();
      flushList();
    } else if (bullet || numbered) {
      flushPara();
      const ordered = !!numbered;
      if (list && list.ordered !== ordered) flushList();
      list ??= { ordered, items: [] };
      list.items.push((bullet ?? numbered)![1]);
    } else {
      flushList();
      // Judul markdown (#) dari LLM ditampilkan sebagai paragraf tebal.
      const heading = line.match(/^#{1,6}\s+(.*)$/);
      if (heading) {
        flushPara();
        para.push(`**${heading[1]}**`);
        flushPara();
      } else para.push(line);
    }
  }
  flushPara();
  flushList();
  return <div className="flex flex-col gap-3">{blocks}</div>;
}
