# GDC Quick Start Guide

5분 안에 GDC를 시작할 수 있는 빠른 가이드입니다.

## 1. 프로젝트 초기화

```bash
# 프로젝트 디렉토리에서 실행
gdc init --language csharp

# 생성되는 구조:
# .gdc/
# ├── config.yaml      # 프로젝트 설정
# ├── graph.db         # SQLite 데이터베이스
# └── nodes/           # 노드 명세 저장소
```

## 2. 첫 번째 노드 생성

```bash
# 인터페이스 노드 생성
gdc node create IPlayerInput --type interface

# 클래스 노드 생성
gdc node create PlayerController --type class --layer application
```

## 3. 명세 작성

`.gdc/nodes/PlayerController.yaml` 편집:

```yaml
schema_version: "1.0"

node:
  id: "PlayerController"
  type: "class"
  layer: "application"

responsibility:
  summary: "플레이어 입력 처리 및 캐릭터 이동 관리"

interface:
  methods:
    - name: "Move"
      signature: "void Move(Vector2 direction)"
      description: "주어진 방향으로 캐릭터 이동"
    - name: "Jump"
      signature: "bool Jump()"
      description: "점프 시도"

dependencies:
  - target: "IPlayerInput"
    type: "interface"
    injection: "constructor"
    usage: |
      - GetAxis(): 이동 입력 조회
      - IsPressed(action): 버튼 상태 확인

metadata:
  status: "specified"
  tags: ["core", "gameplay"]
```

## 4. 그래프 동기화 및 검증

```bash
# YAML → DB 동기화
gdc sync
gdc sync --direction code --files src/services/user_service.go
gdc sync --direction code --symbols UserService

# 정합성 검사
gdc check
```

## 5. AI 프롬프트 추출

```bash
# PlayerController 구현을 위한 최적 프롬프트 생성
gdc extract PlayerController

# 결과를 클립보드에 복사
gdc extract PlayerController --clipboard
```

## 6. 그래프 조회

```bash
# 전체 노드 목록
gdc list
gdc query Game.Controllers.PlayerController
gdc query src/Controllers/PlayerController.cs

# 특정 노드 상세 정보
gdc show PlayerController --deps --refs

# 의존성 추적
gdc trace PlayerController

# 그래프 시각화용 데이터 내보내기
gdc graph --format mermaid
```

## 핵심 워크플로우

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  설계/명세   │ --> │  검증/추출   │ --> │  AI 구현    │
│  (YAML)     │     │  (gdc CLI)  │     │  (LLM)      │
└─────────────┘     └─────────────┘     └─────────────┘
       ↑                                       │
       └───────────── 피드백 ──────────────────┘
```

## CLI 명령어 요약

| 명령어 | 설명 |
|--------|------|
| `gdc init` | 프로젝트 초기화 |
| `gdc node create <name>` | 노드 생성 |
| `gdc list` | 전체 노드 목록 |
| `gdc show <node>` | 노드 상세 정보 |
| `gdc trace <node>` | 의존성 추적 |
| `gdc sync` | YAML-DB 동기화 |
| `gdc check` | 정합성 검사 |
| `gdc extract <node>` | AI 프롬프트 생성 |
| `gdc graph` | 그래프 내보내기 |

---

더 자세한 내용은 [SPEC.md](./SPEC.md)를 참조하세요.
