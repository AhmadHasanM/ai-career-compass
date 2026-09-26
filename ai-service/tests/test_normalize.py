import pytest

from src.extraction.normalize import SkillNormalizer, alias_candidates

ALIASES = {"python": 1, "pytorch": 2, "react": 3, "postgresql": 4, "postgres": 4, "aws": 5, "go": 6, "java 17": 7}


@pytest.mark.parametrize(
    "raw,expected",
    [
        ("Python", 1),
        ("  python 3.x ", 1),
        ("Python3.10", None),  # tanpa spasi tidak dianggap versi; masuk unmapped untuk direview
        ("PyTorch (deep learning)", 2),
        ("React framework", 3),
        ("Postgres", 4),
        ("AWS.", 5),
        ("Go 1.22", 6),
        ("Java 17", 7),  # alias persis menang sebelum versi dibuang
        ("Kubernetes", None),
    ],
)
def test_lookup(raw, expected):
    assert SkillNormalizer(ALIASES).lookup(raw) == expected


def test_alias_candidates_order():
    assert alias_candidates("PyTorch 2.0 (framework)")[:2] == ["pytorch 2.0 (framework)", "pytorch 2.0"]


def test_normalize_required_wins_and_unmapped_deduped():
    n = SkillNormalizer(ALIASES).normalize(
        required=["Python", "Kubernetes", "PostgreSQL"],
        preferred=["python", "Postgres", "kubernetes", "LangSmith", "AWS"],
    )
    assert n.skills == {1: "required", 4: "required", 5: "preferred"}
    assert n.unmapped == ["Kubernetes", "LangSmith"]
