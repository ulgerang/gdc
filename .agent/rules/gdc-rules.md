---
description: GDC 프로젝트에서 코드 작업 시 항상 적용되는 규칙
---

# GDC Project Rules

이 규칙들은 GDC (Graph-Driven Codebase) 프로젝트에서 코드를 작성하거나 수정할 때 항상 적용됩니다.

## Core Principles

### 1. 명세 우선 (Spec First)
- 새로운 클래스, 인터페이스, 서비스를 만들기 전에 **YAML 명세를 먼저 작성**하세요
- 기존 코드를 수정할 때는 먼저 해당 노드의 YAML 명세를 확인하세요
- 명세 파일 위치: `.gdc/nodes/<NodeName>.yaml`

### 2. 동기화 유지 (Keep in Sync)
- 코드를 수정한 후에는 반드시 **YAML 명세도 업데이트**하세요
- 동기화 명령: `gdc sync --direction yaml`
- 정합성 검증: `gdc check`

### 3. 의존성 인식 (Dependency Aware)
- 노드를 수정하기 전에 **의존성 그래프를 확인**하세요
- 의존성 확인: `gdc trace <NodeName>`
- 역의존성 확인: `gdc trace <NodeName> --reverse`

## When Implementing Code

1. **항상 먼저 확인할 것**:
   - 해당 노드의 YAML 명세 (`gdc show <NodeName>`)
   - 의존하는 노드들의 YAML 명세 (`gdc extract <NodeName>`)

2. **구현 시 준수할 것**:
   - YAML의 `interface.methods`에 정의된 시그니처를 정확히 따름
   - YAML의 `dependencies`에 명시된 의존성만 사용
   - `responsibility.summary`의 내용을 코드 주석으로 포함

3. **구현 후 할 것**:
   - `gdc sync --direction yaml` 실행
   - `gdc check` 로 정합성 확인
   - YAML의 `metadata.status`를 `implemented`로 업데이트

## When Modifying Code

1. **수정 전**:
   - `gdc trace <NodeName> --reverse`로 영향받는 노드 확인
   - 시그니처 변경 시 모든 의존 노드에 영향 있음을 인지

2. **수정 시**:
   - 인터페이스 시그니처 변경은 신중하게
   - 새 의존성 추가 시 YAML의 `dependencies` 섹션도 업데이트

3. **수정 후**:
   - `gdc sync --direction yaml`
   - `gdc check`
   - 영향받는 다른 노드들도 필요시 업데이트

## File Naming Conventions

- YAML 파일명: `<NodeName>.yaml` (예: `UserService.yaml`)
- 노드 ID와 파일명은 정확히 일치해야 함
- 인터페이스는 `I` 접두사 사용 권장 (예: `IUserRepository`)

## Layer Architecture

노드의 `layer` 필드는 다음 규칙을 따릅니다:

| Layer | 설명 | 의존 가능 대상 |
|-------|------|---------------|
| `domain` | 도메인 엔티티, 값 객체 | 없음 (가장 안쪽) |
| `application` | 유스케이스, 서비스 | domain |
| `infrastructure` | DB, 외부 API | domain, application |
| `presentation` | UI, API 컨트롤러 | application |

## Quick Reference

```bash
# 노드 정보 확인
gdc show <NodeName>

# 의존성 추적
gdc trace <NodeName>
gdc trace <NodeName> --reverse

# 구현 컨텍스트 추출
gdc extract <NodeName>

# 동기화
gdc sync --direction yaml   # YAML → DB
gdc sync --direction code   # Code → YAML

# 검증
gdc check
```
