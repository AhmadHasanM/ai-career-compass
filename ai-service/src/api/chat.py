import json
import logging

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import StreamingResponse
from openai import RateLimitError

from src.api.deps import verify_internal_token
from src.chat.service import ChatRequest, ChatService
from src.llm.client import LLMNotConfiguredError

log = logging.getLogger(__name__)
router = APIRouter(prefix="/internal", dependencies=[Depends(verify_internal_token)])


def get_chat_service(request: Request) -> ChatService:
    svc: ChatService = request.app.state.chat_service
    try:
        svc.ensure_llm()  # gagal cepat (503) sebelum stream dimulai
    except LLMNotConfiguredError as e:
        raise HTTPException(status_code=503, detail=str(e)) from e
    return svc


def sse(event: str, data: dict) -> str:
    return f"event: {event}\ndata: {json.dumps(data, ensure_ascii=False, default=str)}\n\n"


@router.post("/chat")
async def chat(req: ChatRequest, svc: ChatService = Depends(get_chat_service)) -> StreamingResponse:
    """SSE: event `token` berulang, lalu `done` (jawaban, sitasi lengkap, usage) atau `error`."""

    async def events():
        try:
            async for event, data in svc.stream(req):
                yield sse(event, data)
        except RateLimitError:
            log.warning("chat ditolak: kuota/rate limit LLM habis")
            yield sse("error", {"code": "llm_rate_limited",
                                "message": "Kuota model AI sedang habis. Coba lagi beberapa saat lagi."})
        except Exception:  # noqa: BLE001 - header 200 sudah terkirim; laporkan lewat event
            # Detail error hanya di log; pengguna menerima pesan umum.
            log.exception("chat gagal")
            yield sse("error", {"code": "internal_error", "message": "Terjadi kesalahan saat menyusun jawaban."})

    return StreamingResponse(events(), media_type="text/event-stream",
                             headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"})
