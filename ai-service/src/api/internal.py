from uuid import UUID

from fastapi import APIRouter, BackgroundTasks, Depends, HTTPException, status

from src.api.deps import get_processor, verify_internal_token
from src.ingestion.pipeline import JobProcessor

router = APIRouter(prefix="/internal", dependencies=[Depends(verify_internal_token)])


@router.post("/jobs/{job_id}/process", status_code=status.HTTP_202_ACCEPTED)
async def process_job(
    job_id: UUID,
    background: BackgroundTasks,
    processor: JobProcessor = Depends(get_processor),
) -> dict:
    """Ekstraksi, chunking, dan embedding berjalan di background; hasilnya di job_postings.extraction_status."""
    if not await processor.job_exists(job_id):
        raise HTTPException(status_code=404, detail="lowongan tidak ditemukan")
    if not processor.claim(job_id):
        return {"job_id": str(job_id), "status": "already_processing"}
    background.add_task(processor.run_claimed, job_id)
    return {"job_id": str(job_id), "status": "accepted"}
