"""Konfigurasi ai-service: rahasia dari env (.env), parameter pipeline dari config.yaml."""

from functools import lru_cache
from pathlib import Path
from typing import Any

import yaml
from pydantic_settings import BaseSettings, SettingsConfigDict

BASE_DIR = Path(__file__).resolve().parents[2]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=BASE_DIR.parent / ".env", extra="ignore")

    database_url: str
    internal_token: str = ""

    llm_base_url: str = ""
    llm_api_key: str = ""
    llm_model: str = ""

    embedding_provider: str = "local"
    embedding_model: str = "intfloat/multilingual-e5-base"
    embedding_dim: int = 768

    config_path: Path = BASE_DIR / "config.yaml"

    @property
    def pipeline(self) -> dict[str, Any]:
        return load_pipeline_config(self.config_path)


@lru_cache
def load_pipeline_config(path: Path) -> dict[str, Any]:
    with path.open() as f:
        return yaml.safe_load(f) or {}


@lru_cache
def get_settings() -> Settings:
    return Settings()
