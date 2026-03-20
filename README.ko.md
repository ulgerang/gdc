# GDC - Graph-Driven Codebase

**GDC** 는 AI 협업 개발을 위한 스펙 주도 설계 (Specification-Driven Development) 도구입니다.  
소프트웨어 시스템을 그래프 (노드 & 엣지) 로 모델링하여 AI 코드 생성의 정확도를 극대화합니다.

## ✨ 핵심 원칙

- **Single Source of Truth**: YAML 스펙이 모든 설계의 유일한 진실
- **Context Isolation**: AI 에게 최소한의 정확한 컨텍스트만 제공
- **Graph-First Design**: 클래스 (노드) 와 의존성 (엣지) 으로 시스템 표현
- **Opt-in Evidence**: 코드 증거 (구현/테스트/호출자) 는 옵션으로 점진 도입

## 🚀 빠른 시작

### 빌드

```bash
# Go 1.23+ 필요
go build -o gdc ./cmd/gdc

# Windows
go build -o gdc.exe ./cmd/gdc

# Makefile 사용
make build
```

### 사용법

```bash
# 1. 프로젝트 초기화
gdc init
gdc init --language typescript
gdc init --language go --storage distributed

# 2. 노드 생성
gdc node create PlayerController
gdc node create IInputManager --type interface
gdc node create GameService --type service --layer application

# 3. 노드 관리
gdc node delete OldController
gdc node rename PlayerController CharacterController

# 4. YAML 스펙 작성 (.gdc/nodes/*.yaml 편집)

# 5. 동기화 및 검증
gdc sync                              # YAML → DB 동기화
gdc sync --dry-run                    # 변경사항 미리보기
gdc sync --force                      # 강제 전체 동기화
gdc sync --direction code             # 코드 → YAML 추출
gdc sync --direction code --source src/
gdc sync --direction code --files src/services/user_service.go
gdc sync --direction code --dirs src/services --symbols UserService
gdc sync --direction both --strategy merge
gdc sync --timing --profile --profile-output .gdc/sync-profile.json

gdc check                             # 정합성 검사
gdc check --category hash_mismatch    # 카테고리로 필터
gdc check --severity error            # 심각도로 필터
gdc check --verify-impl --fail-on-missing

# 6. 노드 목록 및 상세 정보
gdc list
gdc list --filter "layer=domain"
gdc list --filter "type=interface"
gdc list --format json
gdc show PlayerController
gdc show PlayerController --deps --refs
gdc show IInputManager --full
gdc show IInputManager --interface-only

# 7. AI 프롬프트 생성
gdc extract PlayerController --clipboard
gdc extract PlayerController --output prompt.md
gdc extract PlayerController --template implement

# 8. 코드 증거 포함 프롬프트 (옵트인)
gdc extract PlayerController --with-impl
gdc extract PlayerController --with-impl --with-tests
```

## 📋 주요 명령어

| 명령어 | 설명 |
|--------|------|
| `gdc init` | 프로젝트 초기화 |
| `gdc version` | 버전 정보 표시 |
| `gdc node create <name>` | 노드 생성 |
| `gdc node delete <name>` | 노드 삭제 |
| `gdc node rename <old> <new>` | 노드 이름 변경 |
| `gdc list` | 노드 목록 |
| `gdc show <node>` | 노드 상세 정보 |
| `gdc trace <node>` | 의존성 추적 |
| `gdc trace <node> --reverse` | 역의존성 추적 (이 노드를 참조하는 노드들) |
| `gdc graph` | 그래프 내보내기 (DOT/Mermaid/JSON) |
| `gdc sync` | YAML ↔ DB 동기화 |
| `gdc check` | 정합성 검사 |
| `gdc extract <node>` | AI 구현 프롬프트 생성 |
| `gdc diff <node>` | YAML 스펙과 현재 코드 차이 비교 |
| `gdc stats` | 프로젝트 통계 |
| `gdc search <pattern>` | 코드베이스에서 패턴 검색 |
| `gdc query <symbol>` | 심볼 이름으로 노드 정보 조회 |

## 🔧 글로벌 플래그

| 플래그 | 설명 |
|--------|------|
| `-c, --config` | 설정 파일 경로 (기본값: .gdc/config.yaml) |
| `-v, --verbose` | 상세 출력 |
| `-q, --quiet` | 최소 출력 |
| `--json` | JSON 형식 출력 |
| `--no-color` | 색상 출력 비활성화 |

## 🔍 검색 및 조회 명령어

### gdc search
코드베이스에서 패턴을 검색합니다.

```bash
# 기본 검색
gdc search "PlayerController"

# 파일 패턴 지정
gdc search "TODO" --file-pattern "*.go"

# 정규식 검색
gdc search "func.*Handler" --regex

# 대소문자 구분
gdc search "UserService" --case-sensitive

# 결과 수 제한
gdc search "import" --max-results 20

# 컨텍스트 라인 포함 (grep-like)
gdc search "error" --context 2
```

### gdc trace --reverse
특정 노드를 참조하는 모든 노드를 추적합니다 (역의존성).

```bash
# PlayerController 에 의존하는 모든 노드 표시
gdc trace PlayerController --reverse

# 깊이 제한
gdc trace PlayerController --reverse --depth 2

# 양방향 (의존성 + 역의존성)
gdc trace PlayerController --direction both

# 특정 노드까지의 경로 탐색
gdc trace PlayerController --to DatabaseService
```

### gdc query
노드 ID, qualified name, 파일 경로, 부분 문자열로 조회할 수 있습니다.
결과에는 canonical ID, spec 경로, 구현 파일 경로, 매칭 방식 같은 provenance가 함께 출력됩니다.
노드 이름으로 상세 정보를 조회합니다. 퍼지 매칭을 지원합니다.

```bash
# 기본 조회
gdc query PlayerController
gdc query Game.Controllers.PlayerController
gdc query src/Controllers/PlayerController.cs
gdc query Player

# JSON 형식 출력
gdc query PlayerController --format json

# YAML 형식 출력
gdc query PlayerController --format yaml

# 상세 정보 (메타데이터, 구현 목록 포함)
gdc query PlayerController --verbose
```

### gdc extract (확장 옵션)
AI 프롬프트 생성 시 코드 증거를 옵트인으로 포함할 수 있습니다.

```bash
# 기본 프롬프트 (스펙 + 의존성 인터페이스만)
gdc extract PlayerController

# 구현 코드 포함
gdc extract PlayerController --with-impl

# 관련 테스트 포함
gdc extract PlayerController --with-tests

# 호출자/참조 정보 포함
gdc extract PlayerController --with-callers

# 모든 코드 증거 포함
gdc extract PlayerController --with-impl --with-tests --with-callers

# 클립보드로 복사
gdc extract PlayerController --with-impl --clipboard

# 파일로 저장
gdc extract PlayerController --output prompt.md

# 다른 템플릿 사용
gdc extract PlayerController --template review
```

### gdc graph
의존성 그래프를 다양한 형식으로 내보냅니다.

```bash
# Mermaid 형식 (기본값)
gdc graph

# Graphviz DOT 형식
gdc graph --format dot --output graph.dot

# JSON 형식
gdc graph --format json > graph.json
```

### gdc diff
저장된 YAML 스펙과 현재 구현 코드의 차이를 비교합니다.

```bash
# 스펙과 구현 비교
gdc diff Agent
```

### gdc stats
프로젝트 통계를 표시합니다.

```bash
# 통계 표시
gdc stats
```

### gdc check
그래프 무결성을 검증하고 문제를 확인합니다.

검증 카테고리:
- `missing_ref` - 존재하지 않는 노드 참조
- `hash_mismatch` - 계약 해시 불일치
- `cycle` - 순환 의존성
- `orphan` - 어디에서도 참조되지 않는 노드
- `impl_missing` - file_path 누락 또는 코드에서 심볼을 찾지 못함
- `impl_mismatch` - 스펙 멤버와 구현이 일치하지 않음
- `layer_violation` - 아키텍처 레이어 위반
- `srp_violation` - 의존성이 너무 많음 (SRP 위반)

```bash
# 모든 검사 실행
gdc check

# 카테고리로 필터
gdc check --category hash_mismatch

# 심각도로 필터
gdc check --severity error

# 자동 수정
gdc check --fix
```

## 🔧 파서

GDC 는 소스 코드에서 노드 정보를 추출하기 위한 다중 언어 파서를 내장하고 있습니다.

### 지원 언어

| 언어 | Regex 파서 | Tree-sitter 파서 |
|------|:----------:|:----------------:|
| Go | ✅ 기본 | - |
| C# | ✅ 기본 | ✅ (빌드 태그) |
| TypeScript | ✅ 기본 | ✅ (빌드 태그) |

### 파서 기능

- **클래스/인터페이스 감지**: 타입 선언, 상속, 구현 관계 추출
- **메서드/프로퍼티 추출**: 시그니처, 접근 제어자, async/static 수식어
- **의존성 자동 감지**: 생성자 주입, 필드 주입 패턴 인식
- **어트리뷰트/데코레이터**: C# 어트리뷰트, TypeScript 데코레이터 추출
- **JSDoc/XML 문서**: 문서 주석에서 설명 추출

### Tree-sitter 파서 사용

```bash
# Tree-sitter 기반 파서로 빌드 (더 정확한 파싱)
go build -tags treesitter -o gdc ./cmd/gdc
```

## 📁 프로젝트 구조

```
.gdc/                            # GDC 프로젝트 설정
├── config.yaml                  # 프로젝트 설정
├── graph.db                     # SQLite 인덱스 (gitignore)
├── nodes/                       # 노드 명세 YAML
│   ├── IInputManager.yaml
│   └── PlayerController.yaml
└── templates/                   # 프롬프트 템플릿
    └── implement.md.j2

cmd/gdc/                         # CLI 진입점
internal/
├── cli/                         # CLI 명령어 정의
│   ├── root.go                  # 루트 명령어 및 글로벌 플래그
│   ├── extract.go               # extract 명령어 (AI 프롬프트 생성)
│   ├── search.go                # search 명령어 (패턴 검색)
│   ├── query.go                 # query 명령어 (심볼 조회)
│   └── trace.go                 # trace 명령어 (의존성/역의존성 추적)
├── extract/                     # 컨텍스트 어셈블리 엔진
│   ├── context_assembler.go     # 오케스트레이터 (Hexagonal Architecture)
│   ├── impl_loader.go           # 구현 코드 로더
│   ├── test_matcher.go          # 테스트 파일 매처
│   ├── caller_resolver.go       # 호출자 리졸버
│   └── output_formatter.go      # 출력 포매터
├── parser/                      # 소스 코드 파서
│   ├── csharp_parser.go         # C# Regex 파서
│   ├── csharp_parser_treesitter.go  # C# Tree-sitter 파서
│   ├── typescript_parser.go     # TypeScript Regex 파서
│   └── typescript_parser_treesitter.go  # TypeScript Tree-sitter 파서
├── search/                      # 검색 인프라
│   ├── interface.go             # 검색 인터페이스 정의
│   └── index_check.go           # 인덱스 상태 체크 (graceful degradation)
├── config/                      # 설정 관리
└── node/                        # 노드 스펙 모델

fixtures/                        # 파서 테스트 픽스처
scripts/                         # 유틸리티 스크립트
└── benchmark_baseline.sh        # 성능 벤치마크 베이스라인
tests/
└── integration/                 # 통합 테스트
```

## 📖 문서

- [📘 SPEC.md](docs/SPEC.md) - 전체 명세서
- [🚀 QUICKSTART.md](docs/QUICKSTART.md) - 빠른 시작 가이드
- [🧭 CODERLM_INTEGRATION.md](docs/CODERLM_INTEGRATION.md) - CodeRLM 접근법 기반 GDC 확장 전략
- [📄 Node Schema](docs/schemas/node-schema.yaml) - 노드 스키마
- [🗄️ DB Schema](docs/schemas/database-schema.sql) - 데이터베이스 스키마

## 🛠 개발

```bash
# 의존성 설치
go mod tidy

# 빌드
make build

# 전체 테스트
make test

# 단계별 검증 테스트
make test-p1    # P1: 기본 기능 검증
make test-p3    # P3: 파서 강화 (C#/TypeScript)
make test-p4    # P4: 검색/조회/추적 명령어

# 정리
make clean
```

## 📄 라이선스

MIT License
