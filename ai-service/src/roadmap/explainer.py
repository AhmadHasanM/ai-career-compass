"""Penjelasan roadmap per node (alasan + estimasi durasi) dari LLM."""

from __future__ import annotations

from pydantic import BaseModel, Field, ValidationError

from src.llm.client import ChatModel
from src.llm.json_call import JSONResult, chat_json, load_json_object
from src.prompts.roadmap import SYSTEM_PROMPT, build_user_prompt


class ExplainProfile(BaseModel):
    education: str | None = None
    current_job: str | None = None
    hours_per_week: int = Field(default=5, ge=1, le=80)
    skills: list[str] = Field(default_factory=list)


class ExplainNode(BaseModel):
    skill_id: int
    name: str
    category: str
    stage: str
    demand_pct: float | None = None
    prerequisites: list[str] | None = None
    required_for: list[str] | None = None


class ExplainRequest(BaseModel):
    role: str
    profile: ExplainProfile
    nodes: list[ExplainNode] = Field(min_length=1, max_length=60)


class ExplainedNode(BaseModel):
    skill_id: int
    rationale: str = Field(min_length=1)
    est_weeks: float = Field(gt=0, le=52)


class _LLMOutput(BaseModel):
    nodes: list[ExplainedNode]


def parse_explanation(content: str, allowed_ids: set[int]) -> list[ExplainedNode]:
    raw = load_json_object(content)
    try:
        out = _LLMOutput.model_validate(raw)
    except ValidationError as e:
        issues = "; ".join(f"{'.'.join(map(str, err['loc']))}: {err['msg']}" for err in e.errors())
        raise ValueError(f"tidak sesuai skema: {issues}") from e
    # Backend Go juga memvalidasi; di sini skill di luar input dibuang lebih awal.
    nodes, seen = [], set()
    for n in out.nodes:
        if n.skill_id in allowed_ids and n.skill_id not in seen:
            seen.add(n.skill_id)
            nodes.append(n)
    if not nodes:
        raise ValueError("tidak ada skill_id yang cocok dengan node input")
    return nodes


async def explain_roadmap(llm: ChatModel, req: ExplainRequest, *, max_retries: int = 1) -> JSONResult[list[ExplainedNode]]:
    allowed = {n.skill_id for n in req.nodes}
    messages = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": build_user_prompt(req.model_dump(exclude_none=True))},
    ]
    return await chat_json(
        llm, messages, lambda c: parse_explanation(c, allowed),
        max_retries=max_retries, temperature=0.3, max_tokens=300 + 120 * len(req.nodes),
    )
