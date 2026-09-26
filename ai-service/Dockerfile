FROM python:3.12-slim
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PIP_NO_CACHE_DIR=1 HF_HOME=/app/.cache/huggingface
WORKDIR /app
# torch versi CPU dulu, supaya sentence-transformers tidak menarik build CUDA (beberapa GB).
RUN pip install torch==2.14.0 --index-url https://download.pytorch.org/whl/cpu
COPY requirements.txt .
RUN pip install -r requirements.txt && useradd -u 10001 -m app
COPY . .
RUN mkdir -p /app/.cache/huggingface && chown -R app /app/logs /app/.cache
USER app
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
