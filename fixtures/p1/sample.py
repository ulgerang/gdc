from abc import ABC, abstractmethod
from typing import Optional, Protocol


class UserRepository(Protocol):
    """Loads users from persistence."""

    def find_by_id(self, user_id: str) -> "User":
        ...


class AuditSink(ABC):
    @abstractmethod
    def record(self, event: str) -> None:
        ...


class UserService:
    repository: UserRepository

    def __init__(
        self,
        repository: UserRepository,
        audit: Optional[AuditSink] = None,
    ) -> None:
        self.repository = repository
        self.audit = audit

    @property
    def enabled(self) -> bool:
        return True

    @enabled.setter
    def enabled(self, value: bool) -> None:
        pass

    async def get_user(self, user_id: str) -> "User":
        return self.repository.find_by_id(user_id)

    def _reset_cache(self) -> None:
        pass


async def build_user_service(repository: UserRepository) -> UserService:
    return UserService(repository)
