import hmac

from fastapi import Header, HTTPException, Request

from src.ingestion.pipeline import JobProcessor
from src.ingestion.resources import ResourceEmbedder
from src.utils.config import get_settings


def verify_internal_token(x_internal_token: str = Header(default="")) -> None:
    """Hanya backend Go yang boleh memanggil /internal (header X-Internal-Token)."""
    expected = get_settings().internal_token
    if not expected:
        raise HTTPException(status_code=503, detail="INTERNAL_TOKEN belum dikonfigurasi")
    if not hmac.compare_digest(x_internal_token.encode(), expected.encode()):
        raise HTTPException(status_code=401, detail="X-Internal-Token tidak valid")


def get_processor(request: Request) -> JobProcessor:
    return request.app.state.processor


def get_resource_embedder(request: Request) -> ResourceEmbedder:
    return request.app.state.resource_embedder
