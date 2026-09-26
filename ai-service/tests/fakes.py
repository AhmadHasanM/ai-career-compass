import hashlib
import json

from src.llm.client import ChatResult


class FakeLLM:
    """Mengembalikan balasan berurutan dari daftar; mencatat pesan yang diterima."""

    def __init__(self, replies: list[str | dict | Exception]) -> None:
        self.replies = list(replies)
        self.calls: list[list[dict]] = []

    async def chat(self, messages, *, temperature=None, max_tokens=None, json_mode=False) -> ChatResult:
        self.calls.append(list(messages))
        reply = self.replies.pop(0)
        if isinstance(reply, Exception):
            raise reply
        content = json.dumps(reply) if isinstance(reply, dict) else reply
        return ChatResult(content=content, model="fake-llm", prompt_tokens=100, completion_tokens=20, latency_ms=1)


class FakeEmbedder:
    model_name = "fake-embedder"

    def __init__(self, dim: int = 768) -> None:
        self.dim = dim

    def _vec(self, text: str) -> list[float]:
        seed = hashlib.sha256(text.encode()).digest()
        return [((seed[i % len(seed)] / 255.0) - 0.5) for i in range(self.dim)]

    async def embed_documents(self, texts):
        return [self._vec(t) for t in texts]

    async def embed_query(self, text):
        return self._vec(text)


class FakeStreamingLLM(FakeLLM):
    """Stream potongan teks tetap, lalu event akhir dengan usage."""

    def __init__(self, pieces: list[str], replies=None) -> None:
        super().__init__(replies or [])
        self.pieces = pieces
        self.stream_calls: list[list[dict]] = []

    async def chat_stream(self, messages, *, temperature=None, max_tokens=None):
        from src.llm.client import StreamDelta

        self.stream_calls.append(list(messages))
        for p in self.pieces:
            yield StreamDelta(text=p)
        yield StreamDelta(done=True, model="fake-llm", prompt_tokens=321, completion_tokens=45)
