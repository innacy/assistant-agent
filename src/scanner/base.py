from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional


@dataclass
class RawItem:
    """An unprocessed item from a data source before extraction."""

    source: str
    source_id: str
    subject: str
    body: str
    sender: str = ""
    date: Optional[datetime] = None
    metadata: dict = field(default_factory=dict)


class BaseScanner(ABC):
    @abstractmethod
    async def authenticate(self) -> None:
        ...

    @abstractmethod
    async def scan(self) -> list[RawItem]:
        ...
