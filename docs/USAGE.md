# GDC CLI 사용 가이드

현재 문서는 저장소의 최신 CLI 동작(`gdc version 1.0.0-dev`) 기준으로 정리되어 있습니다.

## 목차

- [전역 옵션](#전역-옵션)
- [명령어 목록](#명령어-목록)
  - [gdc init](#gdc-init)
  - [gdc node](#gdc-node)
  - [gdc list](#gdc-list)
  - [gdc show](#gdc-show)
  - [gdc sync](#gdc-sync)
  - [gdc check](#gdc-check)
  - [gdc extract](#gdc-extract)
  - [gdc preflight](#gdc-preflight)
  - [gdc trace](#gdc-trace)
  - [gdc graph](#gdc-graph)
  - [gdc diff](#gdc-diff)
  - [gdc search](#gdc-search)
  - [gdc query](#gdc-query)
  - [gdc stats](#gdc-stats)
  - [gdc deps](#gdc-deps)
  - [gdc refs](#gdc-refs)
  - [gdc context](#gdc-context)
  - [gdc version](#gdc-version)
  - [gdc completion](#gdc-completion)

---

## 전역 옵션

모든 GDC 명령어에서 사용할 수 있는 공통 옵션입니다.

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--config` | `-c` | 설정 파일 경로 | `.gdc/config.yaml` |
| `--verbose` | `-v` | 상세 출력 모드 | `false` |
| `--quiet` | `-q` | 최소 출력 모드 | `false` |
| `--json` | - | JSON 형식 출력 (모든 --format 지원 명령어에 적용) | `false` |
| `--no-color` | - | 컬러 출력 비활성화 | `false` |

---

## 명령어 목록

현재 루트 명령은 다음과 같습니다.

- `check`
- `completion`
- `diff`
- `extract`
- `graph`
- `init`
- `list`
- `node`
- `query`
- `search`
- `show`
- `stats`
- `sync`
- `trace`
- `deps`
- `refs`
- `context`
- `version`

### gdc init

새로운 GDC 프로젝트를 초기화합니다.

```bash
gdc init [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--language` | `-l` | 주 프로그래밍 언어 (`csharp`, `typescript`, `go`, `rust`, `python`, `java`) | `csharp` |
| `--storage` | `-s` | 저장 모드 (`centralized`, `distributed`) | `centralized` |

예시:

```bash
gdc init
gdc init --language typescript
gdc init --language rust
gdc init --language python
gdc init --language go --storage distributed
```

### gdc node

노드 명세를 관리합니다.

하위 명령:

- `gdc node create <name>`
- `gdc node delete <name>`
- `gdc node delete <name> --force`
- `gdc node rename <old-name> <new-name>`

대표 예시:

```bash
gdc node create PlayerController
gdc node create IInputManager --type interface
gdc node create GameService --type service --layer application
gdc node delete OldController
gdc node delete OldController --force
gdc node rename PlayerController CharacterController
```

`node delete`는 다른 노드가 대상을 참조하면 참조 노드 목록을 표시하고 삭제를
거부합니다. `--force`를 사용하면 해당 의존 참조도 함께 제거합니다. `node rename`은
모든 YAML 의존 대상과 파생 DB 인덱스를 갱신합니다. 두 명령에서 DB 갱신만 실패한
경우 YAML이 원본이며, 안내에 따라 `gdc sync`로 인덱스를 복구할 수 있습니다.

### gdc list

프로젝트의 노드 목록을 조회합니다.

```bash
gdc list [옵션]
```

별칭: `gdc ls`

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--filter` | `-f` | 필터 표현식 (`layer=domain` 등) | - |
| `--sort` | `-s` | 정렬 기준 (`name`, `type`, `layer`, `status`) | `name` |
| `--format` | - | 출력 형식 (`table`, `json`, `minimal`) | `table` |

예시:

```bash
gdc list
gdc list --filter "layer=domain"
gdc list --filter "type=interface"
gdc list --format json
```

### gdc show

특정 노드의 상세 정보를 조회합니다.

```bash
gdc show <node> [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 |
|------|--------|------|
| `--deps` | `-d` | 의존성 표시 |
| `--refs` | `-r` | 이 노드를 참조하는 노드 표시 |
| `--full` | `-F` | 전체 명세 표시 |
| `--interface-only` | `-i` | 인터페이스만 표시 |
| `--format` | - | 출력 형식 (`text`, `json`) | `text` |

예시:

```bash
gdc show PlayerController
gdc show PlayerController --deps --refs
gdc show IInputManager --full
gdc show PlayerController --format json
```

### gdc sync

YAML 명세와 데이터베이스 또는 코드를 동기화합니다.

```bash
gdc sync [옵션]
```

방향 옵션:

- `yaml`: YAML 명세를 DB 인덱스로 동기화
- `code`: 코드에서 시그니처를 추출해 YAML 갱신
- `both`: 코드 동기화 후 DB 인덱스까지 갱신
- `spec`: 향후 spec-to-code 용도 예약

주요 옵션:

| 옵션 | 축약형 | 설명 |
|------|--------|------|
| `--dry-run` | `-n` | 변경사항 미리보기 |
| `--direction` | `-d` | 동기화 방향 |
| `--source` | `-s` | 코드 추출용 소스 디렉터리 |
| `--files` | - | 특정 파일만 동기화 |
| `--dirs` | - | 특정 디렉터리만 동기화 |
| `--symbols` | - | 특정 심볼만 동기화 |
| `--auto-status` | - | 파일 경로가 있는 code-sync 노드를 `implemented`로 설정 |
| `--strategy` | - | `both` 모드 전략 (`code-first`, `spec-first`, `merge`) |
| `--conflict-log` | - | drift/conflict 로그 파일 경로 |
| `--timing` | - | 타이밍 정보 출력 |
| `--profile` | - | JSON 프로파일 생성 |
| `--profile-output` | - | 프로파일 출력 경로 |

예시:

```bash
gdc sync
gdc sync --dry-run
gdc sync --direction code --source src/
gdc sync --direction code --files src/services/user_service.go
gdc sync --direction code --dirs src/services --symbols UserService
gdc sync --direction both --strategy merge
gdc sync --timing --profile --profile-output .gdc/sync-profile.json
```

### gdc check

그래프 무결성과 구현 드리프트를 검증합니다.

```bash
gdc check [옵션]
```

검증 카테고리:

- `missing_ref`
- `hash_mismatch`
- `cycle`
- `orphan`
- `impl_missing`
- `impl_mismatch`
- `layer_violation`
- `srp_violation`

주요 옵션:

| 옵션 | 설명 |
|------|------|
| `--category` | 카테고리 필터 |
| `--severity` | 심각도 필터 (`error`, `warning`, `info`) |
| `--fix` | 자동 수정 시도 |
| `--verify-impl` | `file_path` 및 인터페이스가 실제 코드에 구현되어 있는지 검증 |
| `--fail-on-missing` | 구현 불일치를 에러로 처리 |
| `--ci-mode` | CI 친화적 출력 사용 |
| `--exit-on-warning` | warning 발생 시 비정상 종료 |
| `--max-warnings` | warning 임계치 |
| `--no-orphan-info` | orphan info 출력 생략 |
| `--orphan-filter` | 특정 패턴 orphan info 필터 |
| `--layer-strict` | layer violation을 에러로 처리 |
| `--format` | 출력 형식 (`text`, `json`) (기본값: `text`) |

예시:

```bash
gdc check
gdc check --category hash_mismatch
gdc check --severity error
gdc check --verify-impl
gdc check --verify-impl --fail-on-missing
gdc check --ci-mode --max-warnings 5
gdc check --layer-strict
gdc check --format json
```

schema 1.1의 `module` 노드는 `interface.types`를 실제 구현 심볼 목록으로
사용합니다. 따라서 모듈 노드 ID와 같은 가상 래퍼 타입은 필요하지 않습니다.
C# 검증은 여러 타입이 있는 파일에서 이름이 맞는 generic 타입을 선택하고,
여러 줄 선언과 overload를 구분해 검사합니다.
`gdc diff`도 동일한 module 바인딩을 사용합니다. `extract`처럼 노드 입력을 받는
명령은 정확한 YAML 파일 stem 또는 모호하지 않은 canonical/bare/kebab-case 노드
ID를 허용합니다.

### gdc extract

AI 구현용 프롬프트를 생성합니다.

```bash
gdc extract <node> [옵션]
```

주요 옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--template` | `-t` | 프롬프트 템플릿 (`implement`, `review`, `test`) | `implement` |
| `--output` | `-o` | 출력 파일 경로 | stdout |
| `--depth` | `-d` | 의존성 포함 깊이 | `1` |
| `--include-logic` | - | 내부 로직 명세 포함 | `false` |
| `--clipboard` | - | 클립보드에 복사 | `false` |
| `--with-impl` | - | 구현 코드 포함 | `false` |
| `--with-tests` | - | 관련 테스트 포함 | `false` |
| `--with-callers` | - | 호출자/참조 정보 포함 | `false` |
| `--for-implementation` | - | schema 1.1/1.2 계약 완결성을 검증하고 source-free 전체 의존 계약 패킷 생성 | `false` |
| `--profile` | - | schema 1.2 구현 프로파일 선택 (여러 프로파일일 때 필수) | - |
| `--format` | - | 출력 형식 (`text`, `json`) | `text` |

예시:

```bash
gdc extract PlayerController
gdc extract PlayerController --clipboard
gdc extract PlayerController --output prompt.md
gdc extract PlayerController --with-impl
gdc extract PlayerController --with-impl --with-tests --with-callers
gdc extract PlayerController --format json
gdc extract PlayerController --for-implementation
gdc extract PlayerController --for-implementation --profile headless
```

`--for-implementation`은 전체 전이 의존성을 깊이 제한 없이 포함하며 대상과 모든
의존 노드의 행동·수명주기·acceptance·필수 멤버·계약 해시를 검사합니다.
schema 1.2 노드는 `sealed` 상태와 선택된 프로파일의 외부 계약·게이트까지
검증하며, 선택된 프로파일과 관련 없는 계약·게이트는 패킷에서 제외됩니다.
저장소 코드 증거를 포함하지 않으므로 `--with-impl`, `--with-tests`,
`--with-callers`와 함께 사용하면 오류가 발생합니다.

### gdc preflight

구현 전에 source-free 계약 준비도를 평가합니다.

```bash
gdc preflight <node> [옵션]
```

옵션:

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--profile` | 구현 프로파일 (여러 프로파일일 때 필수) | - |
| `--phase` | 평가 단계 (`contract`, `implementation`, `verification`, `publish`) | `implementation` |
| `--format` | 출력 형식 (`text`, `json`) | `text` |

예시:

```bash
gdc preflight PlayerController --profile headless
gdc preflight PlayerController --profile headless --phase implementation --format json
gdc preflight PlayerController --phase contract
```

preflight는 구현 소스를 읽지 않고 다음을 보고합니다:

- `contract_complete`: 행동 계약과 프로파일 구조가 완전한가
- `dependency_closure_complete`: 코드 의존 노드와 멤버가 닫혔는가
- `external_contracts_complete`: 선택된 외부 계약이 존재하고 hash가 일치하는가
- `gates_satisfied`: 관련 단계의 게이트가 모두 만족되었는가
- `sealed`: 계약이 sealed 상태인가
- `phase_permitted` / `implementation_permitted`: 해당 단계에서 실행 가능한가
- `missing` / `blocked_by`: 구체적인 누락 항목과 차단 사유

schema 1.2에서 `status: ready`는 수정 가능한 계약 상태를 의미하며 구현을
허용하지 않습니다. `status: sealed`로 전환한 후에만 구현 패킷을 추출할 수
있습니다.

### gdc trace

노드 간 의존성 경로를 추적합니다.

```bash
gdc trace <node> [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--depth` | `-d` | 최대 추적 깊이 (`0` = 무제한) | `0` |
| `--direction` | - | 추적 방향 (`down`, `up`, `both`) | `down` |
| `--reverse` | `-r` | 역의존성 추적 (`--direction up` 별칭) | `false` |
| `--to` | - | 특정 노드까지의 경로 찾기 | - |
| `--format` | - | 출력 형식 (`text`, `json`) | `text` |

예시:

```bash
gdc trace PlayerController
gdc trace PlayerController --depth 2
gdc trace PlayerController --reverse
gdc trace PlayerController --direction both
gdc trace PlayerController --to DatabaseService
gdc trace PlayerController --format json
```

### gdc graph

의존성 그래프를 여러 형식으로 내보냅니다.

```bash
gdc graph [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--format` | `-f` | 출력 형식 (`dot`, `json`, `mermaid`, `html`) | `mermaid` |
| `--output` | `-o` | 출력 파일 경로 | stdout |
| `--interactive` | - | Mermaid 기반 HTML 뷰어 생성 | `false` |
| `--layer-violations` | - | 레이어 위반 엣지 강조 | `false` |
| `--violations-only` | - | 위반 노드/엣지만 출력 | `false` |

예시:

```bash
gdc graph
gdc graph --format dot --output graph.dot
gdc graph --interactive --output graph.html
gdc graph --layer-violations
gdc graph --violations-only
```

### gdc diff

저장된 YAML 스펙과 현재 코드 구현의 차이를 비교합니다.

```bash
gdc diff <node>
```

예시:

```bash
gdc diff Agent
gdc diff Agent --config .gdc/config.yaml
gdc diff Agent --format json
```

### gdc search

소스 파일에서 텍스트 패턴을 검색합니다.

```bash
gdc search <pattern> [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--file-pattern` | `-f` | 검색 대상 파일 패턴 (`*.go`, `*.cs`) | - |
| `--regex` | `-r` | 정규식 검색 | `false` |
| `--case-sensitive` | - | 대소문자 구분 | `false` |
| `--context` | - | 문맥 라인 수 | `0` |
| `--max-results` | `-m` | 최대 결과 수 | `100` |
| `--format` | - | 출력 형식 (`text`, `json`) | `text` |

예시:

```bash
gdc search "PlayerController"
gdc search "TODO" --file-pattern "*.go"
gdc search "func.*Handler" --regex
gdc search "UserService" --case-sensitive
gdc search "error" --max-results 50
gdc search "class" --context 2
gdc search "PlayerController" --format json
```

### gdc query

심볼 이름, 노드 ID, 경로 등으로 노드를 조회합니다.

```bash
gdc query <symbol> [옵션]
```

특징:

- 정확 일치 우선, 이후 부분/퍼지 매칭
- 출력 형식 선택 가능
- `--all`로 모든 랭크 매치를 구조화 배열로 반환
- JSON/YAML 모드에서는 stdout에 구조화 문서만 출력
- `--verbose`로 상세 메타데이터 확인 가능

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--format` | `-f` | 출력 형식 (`text`, `json`, `yaml`) | `text` |
| `--verbose` | `-v` | 상세 정보 출력 | `false` |
| `--all` | - | 모든 매치를 랭크 순서로 반환 | `false` |

예시:

```bash
gdc query PlayerController
gdc query IInputManager
gdc query Player --format json
gdc query Controller --format yaml
gdc query UserService --verbose
gdc query src/service.go --all --format json
```

### gdc stats

프로젝트 통계를 출력합니다.

```bash
gdc stats
gdc stats --format json
```

### gdc deps

노드의 직접 및 전이적 의존성을 나열합니다. 항상 JSON으로 출력됩니다.

```bash
gdc deps <node-id> [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--depth` | `-d` | 의존성 깊이 (1 = 직접 의존성만) | `1` |
| `--transitive` | - | 모든 전이적 의존성 평탄화 (중복 제거) | `false` |

예시:

```bash
gdc deps PlayerController
gdc deps PlayerController --depth 2
gdc deps PlayerController --transitive
```

### gdc refs

특정 노드를 참조(의존)하는 모든 노드를 나열합니다. 항상 JSON으로 출력됩니다.

```bash
gdc refs <node-id> [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--depth` | `-d` | 참조 깊이 (1 = 직접 참조만) | `1` |

예시:

```bash
gdc refs IInputManager
gdc refs IInputManager --depth 2
```

### gdc context

노드의 전체 추출 컨텍스트를 JSON으로 반환합니다 — 명세, 의존성, 선택적 증거 포함.

```bash
gdc context <node-id> [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--depth` | `-d` | 의존성 깊이 | `1` |
| `--with-impl` | - | 구현 코드 증거 포함 | `false` |
| `--with-tests` | - | 테스트 파일 증거 포함 | `false` |
| `--with-callers` | - | 호출자/참조 증거 포함 | `false` |

예시:

```bash
gdc context PlayerController
gdc context PlayerController --depth 2
gdc context PlayerController --with-impl --with-tests --with-callers
```

### gdc version

현재 GDC 버전 정보를 출력합니다.

```bash
gdc version
```

현재 개발 브랜치 예시 출력:

```text
gdc version 1.0.0-dev (built unknown)
```

릴리스 빌드에서는 `built` 값이 빌드 날짜로 치환될 수 있습니다.

### gdc completion

쉘 자동완성 스크립트를 생성합니다.

```bash
gdc completion [bash|zsh|fish|powershell]
```

사용 중인 쉘에 맞춰 저장하거나 프로필에서 로드해 사용할 수 있습니다.
