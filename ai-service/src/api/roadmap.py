from fastapi import APIRouter, Depends, HTTPException

from src.api.deps import verify_internal_token
from src.llm.client import LLMNotConfiguredError
from src.llm.json_call import JSONCallError
from src.roadmap.explainer import ExplainRequest, explain_roadmap

router = APIRouter(prefix="/internal", dependencies=[Depends(verify_internal_token)])


def get_llm():
    from src.ingestion.factory import get_llm as factory_llm

    try:
        return factory_llm()
    except LLMNotConfiguredError as e:
        raise HTTPException(status_code=503, detail=str(e)) from e


@router.post("/roadmap/explain")
async def roadmap_explain(req: ExplainRequest, llm=Depends(get_llm)) -> dict:
    """Alasan + estimasi durasi per node. Backend Go memakai fallback jika endpoint ini gagal."""
    try:
        res = await explain_roadmap(llm, req)
    except JSONCallError as e:
        raise HTTPException(status_code=502, detail=str(e)) from e
    return {"model": res.model, "nodes": [n.model_dump() for n in res.data]}
