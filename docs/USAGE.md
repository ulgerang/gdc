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
  - [gdc trace](#gdc-trace)
  - [gdc graph](#gdc-graph)
  - [gdc diff](#gdc-diff)
  - [gdc search](#gdc-search)
  - [gdc query](#gdc-query)
  - [gdc stats](#gdc-stats)
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
| `--json` | - | JSON 형식 출력 | `false` |
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
- `version`

### gdc init

새로운 GDC 프로젝트를 초기화합니다.

```bash
gdc init [옵션]
```

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--language` | `-l` | 주 프로그래밍 언어 (`csharp`, `typescript`, `go`, `python`, `java`) | `csharp` |
| `--storage` | `-s` | 저장 모드 (`centralized`, `distributed`) | `centralized` |

예시:

```bash
gdc init
gdc init --language typescript
gdc init --language go --storage distributed
```

### gdc node

노드 명세를 관리합니다.

하위 명령:

- `gdc node create <name>`
- `gdc node delete <name>`
- `gdc node rename <old-name> <new-name>`

대표 예시:

```bash
gdc node create PlayerController
gdc node create IInputManager --type interface
gdc node create GameService --type service --layer application
gdc node delete OldController
gdc node rename PlayerController CharacterController
```

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

예시:

```bash
gdc show PlayerController
gdc show PlayerController --deps --refs
gdc show IInputManager --full
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

예시:

```bash
gdc check
gdc check --category hash_mismatch
gdc check --severity error
gdc check --verify-impl
gdc check --verify-impl --fail-on-missing
gdc check --ci-mode --max-warnings 5
gdc check --layer-strict
```

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

예시:

```bash
gdc extract PlayerController
gdc extract PlayerController --clipboard
gdc extract PlayerController --output prompt.md
gdc extract PlayerController --with-impl
gdc extract PlayerController --with-impl --with-tests --with-callers
```

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

예시:

```bash
gdc trace PlayerController
gdc trace PlayerController --depth 2
gdc trace PlayerController --reverse
gdc trace PlayerController --direction both
gdc trace PlayerController --to DatabaseService
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

예시:

```bash
gdc search "PlayerController"
gdc search "TODO" --file-pattern "*.go"
gdc search "func.*Handler" --regex
gdc search "UserService" --case-sensitive
gdc search "error" --max-results 50
gdc search "class" --context 2
```

### gdc query

심볼 이름, 노드 ID, 경로 등으로 노드를 조회합니다.

```bash
gdc query <symbol> [옵션]
```

특징:

- 정확 일치 우선, 이후 부분/퍼지 매칭
- 출력 형식 선택 가능
- `--verbose`로 상세 메타데이터 확인 가능

옵션:

| 옵션 | 축약형 | 설명 | 기본값 |
|------|--------|------|--------|
| `--format` | `-f` | 출력 형식 (`text`, `json`, `yaml`) | `text` |
| `--verbose` | `-v` | 상세 정보 출력 | `false` |

예시:

```bash
gdc query PlayerController
gdc query IInputManager
gdc query Player --format json
gdc query Controller --format yaml
gdc query UserService --verbose
```

### gdc stats

프로젝트 통계를 출력합니다.

```bash
gdc stats
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
