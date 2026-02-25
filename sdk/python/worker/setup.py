import os
from pathlib import Path

from setuptools import find_packages, setup


def read_version() -> str:
    raw = (os.getenv("GITHOOK_PY_VERSION") or "").strip()
    if raw.startswith("v"):
        raw = raw[1:]
    return raw or "0.0.10"


def read_readme() -> str:
    path = Path(__file__).with_name("README.md")
    return path.read_text(encoding="utf-8")

setup(
    name="relaymesh-githook",
    version=read_version(),
    description="Relaymesh Githook worker SDK",
    long_description=read_readme(),
    long_description_content_type="text/markdown",
    python_requires=">=3.9",
    packages=find_packages(where=".", include=["relaymesh_githook*", "cloud*"]),
    install_requires=[
        "protobuf>=4.21.0",
    ],
    extras_require={
        "amqp": ["relaybus-amqp"],
        "kafka": ["relaybus-kafka"],
        "nats": ["relaybus-nats"],
        "all": ["relaybus-amqp", "relaybus-kafka", "relaybus-nats"],
    },
)
