"use client";

import "@xyflow/react/dist/style.css";

import { Background, Controls, Handle, MarkerType, Position, ReactFlow, type Edge, type Node, type NodeProps } from "@xyflow/react";
import { useMemo } from "react";
import type { Roadmap, RoadmapNode } from "@/lib/api";

const COL_W = 248;
const ROW_H = 104;
const NODE_W = 208;

type SkillNodeData = { node: RoadmapNode; selected: boolean; onSelect: (id: number) => void };

function stageIndex(stage: string) {
  const n = Number(stage.replace(/\D+/g, ""));
  return Number.isFinite(n) && n > 0 ? n - 1 : 0;
}

function SkillNode({ data }: NodeProps<Node<SkillNodeData>>) {
  const { node, selected, onSelect } = data;
  const done = node.status === "done";
  return (
    <button
      type="button"
      onClick={() => onSelect(node.skill_id)}
      aria-pressed={selected}
      style={{ width: NODE_W }}
      className={
        "flex flex-col gap-1.5 rounded-panel border bg-surface px-3 py-2.5 text-left " +
        (selected ? "border-compass ring-[3px] ring-compass-soft" : "border-line-strong hover:border-ink")
      }
    >
      <Handle type="target" position={Position.Left} className="!size-1.5 !border-0 !bg-line-strong" />
      <span className="flex items-center justify-between gap-2">
        <span className="truncate text-[14px] font-medium text-ink">{node.name}</span>
        <span
          aria-label={done ? "Selesai" : node.status === "in_progress" ? "Berjalan" : "Belum"}
          className={
            "box-border size-2 shrink-0 rounded-full " +
            (done ? "bg-compass" : node.status === "in_progress" ? "border-[1.5px] border-ink" : "border-[1.5px] border-muted")
          }
        />
      </span>
      <span className="flex gap-3 font-mono text-[12px] text-muted">
        <span>{node.demand_pct != null ? `${node.demand_pct}%` : "prasyarat"}</span>
        {node.est_weeks != null && <span>{node.est_weeks} mgg</span>}
      </span>
      <Handle type="source" position={Position.Right} className="!size-1.5 !border-0 !bg-line-strong" />
    </button>
  );
}

const nodeTypes = { skill: SkillNode };

export function RoadmapGraph({
  roadmap,
  selected,
  onSelect,
}: {
  roadmap: Roadmap;
  selected: number | null;
  onSelect: (skillId: number) => void;
}) {
  const { nodes, edges, height } = useMemo(() => {
    const rowsPerCol = new Map<number, number>();
    const nodes: Node<SkillNodeData>[] = roadmap.nodes.map((n) => {
      const col = stageIndex(n.stage);
      const row = rowsPerCol.get(col) ?? 0;
      rowsPerCol.set(col, row + 1);
      return {
        id: String(n.skill_id),
        type: "skill",
        position: { x: col * COL_W, y: row * ROW_H },
        data: { node: n, selected: n.skill_id === selected, onSelect },
        draggable: false,
        connectable: false,
      };
    });
    const edges: Edge[] = roadmap.edges.map((e) => {
      const active = e.from_skill_id === selected || e.to_skill_id === selected;
      return {
        id: `${e.from_skill_id}-${e.to_skill_id}`,
        source: String(e.from_skill_id),
        target: String(e.to_skill_id),
        type: "smoothstep",
        markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color: active ? "#1E6B52" : "#C9C5BA" },
        style: { stroke: active ? "#1E6B52" : "#C9C5BA", strokeWidth: active ? 1.75 : 1.25 },
      };
    });
    const maxRows = Math.max(1, ...rowsPerCol.values());
    return { nodes, edges, height: Math.min(640, Math.max(320, maxRows * ROW_H + 96)) };
  }, [roadmap, selected, onSelect]);

  return (
    <div style={{ height }} className="w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15, maxZoom: 1 }}
        minZoom={0.4}
        maxZoom={1.5}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
        aria-label="Graf roadmap: tiap node adalah skill, panah menunjukkan prasyarat"
      >
        <Background gap={16} size={1} color="#E2DFD7" />
        <Controls showInteractive={false} position="bottom-right" />
      </ReactFlow>
    </div>
  );
}
