from types import SimpleNamespace

import pytest

from src.llm.client import OpenAICompatibleLLM
from src.utils.config import Settings


class RejectingCompletions:
    """Meniru endpoint yang menolak parameter tertentu dengan 400, seperti Groq untuk reasoning_effort."""

    def __init__(self, rejected: dict[str, str]):
        self.rejected = rejected
        self.calls: list[dict] = []

    async def create(self, **kwargs):
        self.calls.append(dict(kwargs))
        for param, message in self.rejected.items():
            if param in kwargs:
                err = Exception(f"Error code: 400 - {{'error': {{'message': '{message}'}}}}")
                err.status_code = 400
                raise err
        msg = SimpleNamespace(content='{"ok": true}')
        return SimpleNamespace(choices=[SimpleNamespace(message=msg)], model=kwargs["model"],
                               usage=SimpleNamespace(prompt_tokens=10, completion_tokens=2))


def make_llm(rejected, reasoning="none"):
    s = Settings(database_url="postgres://x", llm_base_url="https://api.example.com/v1", llm_api_key="k",
                 llm_model="m", _env_file=None)
    llm = OpenAICompatibleLLM(s)
    llm.reasoning_effort = reasoning
    fake = RejectingCompletions(rejected)
    llm._client = SimpleNamespace(chat=SimpleNamespace(completions=fake))
    return llm, fake


@pytest.mark.anyio
async def test_rejected_reasoning_effort_is_dropped_and_remembered():
    llm, fake = make_llm({"reasoning_effort": "`reasoning_effort` must be one of `low`, `medium`, or `high`"})
    res = await llm.chat([{"role": "user", "content": "x"}], json_mode=True)
    assert res.content == '{"ok": true}'
    assert "reasoning_effort" in fake.calls[0] and "reasoning_effort" not in fake.calls[1]
    assert fake.calls[1]["response_format"] == {"type": "json_object"}  # parameter lain tetap dikirim
    await llm.chat([{"role": "user", "content": "y"}])
    assert "reasoning_effort" not in fake.calls[2], "setelah ditolak sekali, tidak dikirim lagi"


@pytest.mark.anyio
async def test_multiple_rejected_params_are_all_dropped():
    llm, fake = make_llm({"reasoning_effort": "unsupported reasoning_effort",
                          "response_format": "response_format not supported"})
    res = await llm.chat([{"role": "user", "content": "x"}], json_mode=True)
    assert res.content and len(fake.calls) == 3
    assert "reasoning_effort" not in fake.calls[2] and "response_format" not in fake.calls[2]


@pytest.mark.anyio
async def test_other_errors_are_not_swallowed():
    llm, _ = make_llm({"messages": "messages must not be empty"})
    with pytest.raises(Exception, match="messages must not be empty"):
        await llm.chat([{"role": "user", "content": "x"}])
