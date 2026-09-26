import pytest

from src.utils.config import Settings


@pytest.mark.parametrize(
    "raw,expected",
    [
        ("https://opencode.ai/inference/openai/v1/chat/completions", "https://opencode.ai/inference/openai/v1"),
        ("https://opencode.ai/inference/openai/v1/", "https://opencode.ai/inference/openai/v1"),
        ("https://api.example.com/v1/embeddings", "https://api.example.com/v1"),
        ("https://api.example.com/v1", "https://api.example.com/v1"),
        ("", ""),
    ],
)
def test_llm_base_url_strips_endpoint_path(raw, expected):
    s = Settings(database_url="postgres://x", llm_base_url=raw, _env_file=None)
    assert s.llm_base_url == expected
