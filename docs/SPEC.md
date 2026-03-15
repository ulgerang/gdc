# GDC (Graph-Driven Codebase) CLI Specification

> **Version**: 1.0.0-draft  
> **Last Updated**: 2026-02-03  
> **Status**: Draft

---

## 1. 개요 (Overview)

### 1.1. 프로젝트 목표

**GDC**는 스펙 주도 개발(Specification-Driven Development, SDD) 방법론을 실현하기 위한 CLI 도구입니다. 소프트웨어 시스템을 **그래프(Nodes & Edges)**로 모델링하고, 이를 기반으로 AI 협업 개발을 최적화합니다.

### 1.2. 핵심 원칙

| 원칙 | 설명 |
|------|------|
| **Single Source of Truth** | YAML 명세가 유일한 진실의 원천 |
| **Context Isolation** | AI에게 최소한의 컨텍스트만 제공하여 정확도 극대화 |
| **Graph-First Design** | 시스템을 노드(클래스/모듈)와 엣지(의존성)로 표현 |
| **Incremental Sync** | 변경된 부분만 인덱싱하여 성능 최적화 |

### 1.3. 핵심 구성 요소

```
┌─────────────────────────────────────────────────────────────┐
│                     GDC Architecture                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐  │
│  │  YAML   │ -> │ Indexer │ -> │ SQLite  │ <- │   CLI   │  │
│  │  Specs  │    │         │    │   DB    │    │Commands │  │
│  └─────────┘    └─────────┘    └─────────┘    └─────────┘  │
│       ↑                              ↓                      │
│       └──────── Sync ────────────────┘                      │
│                      ↓                                      │
│              ┌─────────────┐                               │
│              │ LLM Prompt  │                               │
│              │  Generator  │                               │
│              └─────────────┘                               │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 프로젝트 구조 (Project Structure)

### 2.1. GDC 관리 디렉토리

```
project-root/
├── .gdc/                          # GDC 관리 폴더
│   ├── config.yaml                # 프로젝트 설정
│   ├── graph.db                   # SQLite3 데이터베이스 (gitignore)
│   ├── nodes/                     # 노드 명세 파일들
│   │   ├── PlayerController.yaml
│   │   ├── InputManager.yaml
│   │   └── ...
│   └── templates/                 # 프롬프트 템플릿
│       ├── implement.md.j2
│       └── review.md.j2
├── src/                           # 실제 소스 코드
│   ├── PlayerController.cs
│   └── ...
└── .gitignore
```

### 2.2. 설정 파일 스키마 (`config.yaml`)

```yaml
# .gdc/config.yaml
version: "1.0"

project:
  name: "MyProject"
  language: "csharp"          # csharp | typescript | go | python
  source_root: "./src"        # 소스 코드 루트 경로

storage:
  mode: "centralized"         # centralized | distributed
  nodes_dir: ".gdc/nodes"     # mode가 centralized일 때

database:
  path: ".gdc/graph.db"
  wal_mode: true              # Write-Ahead Logging 활성화

indexer:
  watch: false                # 파일 감시 자동 동기화
  ignore_patterns:
    - "*.test.cs"
    - "**/bin/**"

llm:
  default_model: "gpt-4"
  templates_dir: ".gdc/templates"
```

---

## 3. 노드 명세 스키마 (Node Specification Schema)

### 3.1. 전체 스키마 정의

```yaml
# .gdc/nodes/{NodeName}.yaml
schema_version: "1.0"

# ─────────────────────────────────────────────────────────────
# 노드 기본 정보 (Node Metadata)
# ─────────────────────────────────────────────────────────────
node:
  id: "PlayerController"                    # 고유 식별자 (필수)
  type: "class"                             # class | interface | module | service
  layer: "application"                      # domain | application | infrastructure | presentation
  namespace: "Game.Controllers"             # 네임스페이스/패키지
  file_path: "src/Controllers/PlayerController.cs"  # 구현 파일 경로

# ─────────────────────────────────────────────────────────────
# 책임 정의 (Responsibility)
# ─────────────────────────────────────────────────────────────
responsibility:
  summary: "플레이어 입력 처리 및 캐릭터 상태 관리"
  details: |
    - 사용자 입력을 받아 캐릭터 행동으로 변환
    - 이동, 점프, 공격 등 액션 상태 머신 관리
    - 애니메이션 트리거 발생
  invariants:                               # 불변식 (항상 참이어야 하는 조건)
    - "플레이어 속도는 항상 0 이상"
    - "체력은 0에서 maxHealth 사이"

# ─────────────────────────────────────────────────────────────
# 인터페이스 정의 (Public Interface)
# ─────────────────────────────────────────────────────────────
interface:
  constructors:
    - signature: "PlayerController(IInputManager input, IPhysicsEngine physics)"
      description: "의존성 주입을 통한 생성"
  
  methods:
    - name: "Move"
      signature: "void Move(Vector2 direction)"
      description: "주어진 방향으로 이동 처리"
      parameters:
        - name: "direction"
          type: "Vector2"
          description: "정규화된 이동 방향 벡터"
      returns:
        type: "void"
      throws:
        - type: "InvalidOperationException"
          condition: "플레이어가 죽은 상태일 때"
    
    - name: "Jump"
      signature: "bool Jump()"
      description: "점프 시도, 성공 여부 반환"
      returns:
        type: "bool"
        description: "점프 성공 시 true"
      preconditions:
        - "IsGrounded == true"
      postconditions:
        - "velocity.y > 0"

    - name: "TakeDamage"
      signature: "void TakeDamage(int amount, DamageType type)"
      description: "데미지 처리 및 상태 업데이트"
      parameters:
        - name: "amount"
          type: "int"
          constraint: ">= 0"
        - name: "type"
          type: "DamageType"
          enum: ["Physical", "Magical", "True"]

  properties:
    - name: "Health"
      type: "int"
      access: "get"
      description: "현재 체력"
    
    - name: "IsGrounded"
      type: "bool"
      access: "get"
      description: "지면 접촉 여부"

  events:
    - name: "OnDeath"
      signature: "event Action OnDeath"
      description: "플레이어 사망 시 발생"
    
    - name: "OnDamaged"
      signature: "event Action<int, DamageType> OnDamaged"
      description: "데미지를 받았을 때 발생"

# ─────────────────────────────────────────────────────────────
# 의존성 정의 (Dependencies / Edges)
# ─────────────────────────────────────────────────────────────
dependencies:
  - target: "IInputManager"
    type: "interface"                       # interface | class | service
    injection: "constructor"                # constructor | property | method
    usage: |
      - GetMovementAxis(): 이동 방향 입력 조회
      - IsActionPressed(action): 액션 버튼 상태 확인
    contract_hash: "a1b2c3d4"               # 의존 시점의 인터페이스 해시

  - target: "IPhysicsEngine"
    type: "interface"
    injection: "constructor"
    usage: |
      - ApplyForce(force): 물리 힘 적용
      - Raycast(origin, direction): 지면 감지용 레이캐스트
    contract_hash: "e5f6g7h8"

  - target: "IAnimationController"
    type: "interface"
    injection: "property"
    optional: true                          # 선택적 의존성
    usage: |
      - SetTrigger(name): 애니메이션 트리거
      - SetFloat(name, value): 블렌드 파라미터 설정

# ─────────────────────────────────────────────────────────────
# 내부 로직 스펙 (Business Logic Specification)
# ─────────────────────────────────────────────────────────────
logic:
  state_machine:
    initial: "Idle"
    states:
      - name: "Idle"
        transitions:
          - to: "Walking"
            trigger: "movement input detected"
          - to: "Jumping"
            trigger: "Jump() called && IsGrounded"
      - name: "Walking"
        transitions:
          - to: "Idle"
            trigger: "no movement input"
          - to: "Running"
            trigger: "sprint button held"
      - name: "Jumping"
        entry_action: "Apply upward force"
        transitions:
          - to: "Falling"
            trigger: "velocity.y < 0"
      - name: "Falling"
        transitions:
          - to: "Idle"
            trigger: "IsGrounded"

  algorithms:
    - name: "DamageCalculation"
      description: |
        final_damage = base_damage * type_multiplier - defense
        if final_damage < 0: final_damage = 1 (최소 데미지)
      references:
        - "DamageType enum for multipliers"

# ─────────────────────────────────────────────────────────────
# 메타데이터 (Metadata)
# ─────────────────────────────────────────────────────────────
metadata:
  author: "developer@example.com"
  created: "2026-01-15"
  updated: "2026-02-03"
  tags: ["core", "gameplay", "player"]
  status: "implemented"                     # draft | specified | implemented | tested
  impl_hash: "x9y0z1a2"                     # 구현 코드의 최신 해시
  spec_hash: "b3c4d5e6"                     # 이 명세의 해시
```

### 3.2. 인터페이스 노드 스키마

```yaml
# .gdc/nodes/IInputManager.yaml
schema_version: "1.0"

node:
  id: "IInputManager"
  type: "interface"
  layer: "application"
  namespace: "Game.Input"

responsibility:
  summary: "입력 장치 추상화 인터페이스"
  details: |
    다양한 입력 장치(키보드, 게임패드, 터치)를 
    통일된 인터페이스로 제공

interface:
  methods:
    - name: "GetMovementAxis"
      signature: "Vector2 GetMovementAxis()"
      description: "정규화된 이동 축 벡터 반환"
      returns:
        type: "Vector2"
        constraint: "magnitude <= 1.0"
    
    - name: "IsActionPressed"
      signature: "bool IsActionPressed(string actionName)"
      parameters:
        - name: "actionName"
          type: "string"
          examples: ["Jump", "Attack", "Interact"]
      returns:
        type: "bool"
    
    - name: "IsActionJustPressed"
      signature: "bool IsActionJustPressed(string actionName)"
      description: "이번 프레임에 처음 눌렸는지 확인"

# 구현체 목록 (참고용)
implementations:
  - "KeyboardInputManager"
  - "GamepadInputManager"
  - "MobileInputManager"

metadata:
  status: "specified"
  spec_hash: "f7g8h9i0"
```

### 3.3. 언어별 하이브리드 명세 전략 (Language-Specific Hybrid Strategy)

언어마다 고유의 문법과 철학이 다르므로, YAML 명세는 **Header(Universal) + Body(Language-Specific)** 구조로 구성됩니다.

#### 3.3.1. 구조 개요

```
┌─────────────────────────────────────────┐
│ Header (Universal)                      │ ← 그래프 순회, 정합성 검사 (CLI)
│ - node.id, node.type                    │
│ - dependencies[]                        │
│ - metadata.spec_hash, impl_hash         │
├─────────────────────────────────────────┤
│ Body (Language-Specific)                │ ← LLM 코드 생성, 언어 관습 반영
│ - language_spec.*                       │
│ - interface.methods[].access            │
│ - interface.methods[].attributes        │
│ - interface.methods[].exported          │
└─────────────────────────────────────────┘
```

#### 3.3.2. C# (Unity/Enterprise) 명세 예시

```yaml
schema_version: "1.0"

node:
  id: "PlayerController"
  type: "class"
  namespace: "Game.Controllers"

# ═══ Language-Specific Body ═══
language_spec:
  language: "csharp"
  attributes:
    - "[RequireComponent(typeof(Rigidbody))]"
    - "[AddComponentMenu(\"Game/Player Controller\")]"

interface:
  constructors:
    - signature: "PlayerController(IInputManager input)"
      access: "public"
      attributes:
        - "[Inject]"  # DI 프레임워크 특성

  methods:
    - name: "Move"
      signature: "void Move(Vector3 direction)"
      access: "public"
      virtual: true
      attributes:
        - "[MethodImpl(MethodImplOptions.AggressiveInlining)]"

    - name: "TakeDamage"
      signature: "async Task TakeDamage(int amount)"
      access: "public"
      async: true  # C# async/await

  properties:
    - name: "Health"
      type: "int"
      access: "get"
      attributes:
        - "[SerializeField]"
        - "[Range(0, 100)]"
```

#### 3.3.3. Go (Golang) 명세 예시

```yaml
schema_version: "1.0"

node:
  id: "UserService"
  type: "class"

# ═══ Language-Specific Body ═══
language_spec:
  language: "go"
  package: "services"

interface:
  constructors:
    - signature: "NewUserService(repo UserRepository, logger Logger) *UserService"
      exported: true  # Go: 대문자 시작 = exported

  methods:
    - name: "FindByID"
      signature: "FindByID(ctx context.Context, id string) (*User, error)"
      exported: true
      description: "ID로 사용자 조회"

    - name: "validate"
      signature: "validate(user *User) error"
      exported: false  # 소문자 시작 = unexported (private)

  properties:
    - name: "repo"
      type: "UserRepository"
      exported: false  # 소문자 = private 필드
    
    - name: "Logger"
      type: "Logger"
      exported: true   # 대문자 = public 필드
```

#### 3.3.4. TypeScript 명세 예시

```yaml
schema_version: "1.0"

node:
  id: "ApiClient"
  type: "class"

# ═══ Language-Specific Body ═══
language_spec:
  language: "typescript"
  module: "./services/api"

interface:
  constructors:
    - signature: "constructor(private readonly config: ApiConfig)"
      access: "public"

  methods:
    - name: "fetchUser"
      signature: "fetchUser(id: string): Promise<User>"
      access: "public"
      async: true  # TypeScript: Promise 반환 = async

    - name: "transformResponse"
      signature: "transformResponse<T>(response: Response): T"
      access: "private"
      description: "제네릭 응답 변환"

  properties:
    - name: "baseUrl"
      type: "string"
      readonly: true   # TypeScript: readonly modifier
      access: "get"

    - name: "instance"
      type: "ApiClient"
      static: true     # TypeScript: static property
```

#### 3.3.5. 언어별 필드 매핑 요약

| 필드 | C# | Go | TypeScript | 용도 |
|------|----|----|------------|------|
| `access` | public/private/internal/protected | - | public/private/protected | 접근 제어 |
| `exported` | - | true/false | - | Go 대문자 규칙 |
| `async` | async/await | - | Promise | 비동기 처리 |
| `readonly` | - | - | readonly | 불변 속성 |
| `static` | static | - | static | 정적 멤버 |
| `virtual` | virtual/override | - | - | 다형성 |
| `abstract` | abstract | - | abstract | 추상 멤버 |
| `attributes` | [Attribute] | - | - | 데코레이터/특성 |

---

## 4. 데이터베이스 스키마 (SQLite3 Schema)

### 4.1. 테이블 정의

```sql
-- .gdc/graph.db

-- ═══════════════════════════════════════════════════════════════
-- 노드 테이블 (Nodes)
-- ═══════════════════════════════════════════════════════════════
CREATE TABLE nodes (
    id              TEXT PRIMARY KEY,           -- 노드 식별자
    type            TEXT NOT NULL,              -- class | interface | module | service
    layer           TEXT,                       -- domain | application | infrastructure | presentation
    namespace       TEXT,
    file_path       TEXT,                       -- YAML 파일 경로
    impl_path       TEXT,                       -- 구현 파일 경로
    responsibility  TEXT,                       -- 책임 요약
    status          TEXT DEFAULT 'draft',       -- draft | specified | implemented | tested
    impl_hash       TEXT,                       -- 구현 코드 해시
    spec_hash       TEXT,                       -- YAML 명세 해시
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_nodes_type ON nodes(type);
CREATE INDEX idx_nodes_layer ON nodes(layer);
CREATE INDEX idx_nodes_status ON nodes(status);

-- ═══════════════════════════════════════════════════════════════
-- 엣지 테이블 (Dependencies)
-- ═══════════════════════════════════════════════════════════════
CREATE TABLE edges (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    from_node       TEXT NOT NULL,              -- 의존하는 노드
    to_node         TEXT NOT NULL,              -- 의존 대상 노드
    dependency_type TEXT NOT NULL,              -- interface | class | service
    injection_type  TEXT,                       -- constructor | property | method
    is_optional     BOOLEAN DEFAULT FALSE,
    contract_hash   TEXT,                       -- 의존 시점의 대상 인터페이스 해시
    usage_summary   TEXT,                       -- 사용 방식 요약
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (from_node) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (to_node) REFERENCES nodes(id) ON DELETE CASCADE,
    UNIQUE(from_node, to_node)
);

CREATE INDEX idx_edges_from ON edges(from_node);
CREATE INDEX idx_edges_to ON edges(to_node);

-- ═══════════════════════════════════════════════════════════════
-- 인터페이스 메서드 테이블 (Interface Methods)
-- ═══════════════════════════════════════════════════════════════
CREATE TABLE interfaces (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id         TEXT NOT NULL,
    member_type     TEXT NOT NULL,              -- method | property | event | constructor
    name            TEXT NOT NULL,
    signature       TEXT NOT NULL,
    description     TEXT,
    return_type     TEXT,
    parameters_json TEXT,                       -- JSON 배열로 파라미터 저장
    
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_interfaces_node ON interfaces(node_id);
CREATE INDEX idx_interfaces_name ON interfaces(name);

-- ═══════════════════════════════════════════════════════════════
-- 태그 테이블 (Tags)
-- ═══════════════════════════════════════════════════════════════
CREATE TABLE tags (
    node_id         TEXT NOT NULL,
    tag             TEXT NOT NULL,
    
    PRIMARY KEY (node_id, tag),
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_tags_tag ON tags(tag);

-- ═══════════════════════════════════════════════════════════════
-- 동기화 로그 테이블 (Sync Log)
-- ═══════════════════════════════════════════════════════════════
CREATE TABLE sync_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    action          TEXT NOT NULL,              -- create | update | delete | validate
    target_node     TEXT,
    target_file     TEXT,
    old_hash        TEXT,
    new_hash        TEXT,
    message         TEXT,
    timestamp       DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════
-- 검증 이슈 테이블 (Validation Issues)
-- ═══════════════════════════════════════════════════════════════
CREATE TABLE validation_issues (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    severity        TEXT NOT NULL,              -- error | warning | info
    category        TEXT NOT NULL,              -- missing_ref | hash_mismatch | cycle | srp_violation
    source_node     TEXT,
    target_node     TEXT,
    message         TEXT NOT NULL,
    suggestion      TEXT,
    resolved        BOOLEAN DEFAULT FALSE,
    detected_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_issues_severity ON validation_issues(severity);
CREATE INDEX idx_issues_resolved ON validation_issues(resolved);
```

### 4.2. 주요 쿼리 예시

```sql
-- 1. 특정 노드를 의존하는 모든 노드 조회 (역방향 쿼리)
SELECT n.id, n.type, n.responsibility
FROM nodes n
JOIN edges e ON n.id = e.from_node
WHERE e.to_node = 'IInputManager';

-- 2. 고립된 노드 찾기 (어디서도 참조되지 않는 노드)
SELECT n.id, n.type
FROM nodes n
LEFT JOIN edges e ON n.id = e.to_node
WHERE e.id IS NULL
  AND n.type != 'interface';  -- 인터페이스는 제외

-- 3. 해시 불일치 감지 (의존성 계약 위반)
SELECT 
    e.from_node,
    e.to_node,
    e.contract_hash AS expected_hash,
    n.spec_hash AS current_hash
FROM edges e
JOIN nodes n ON e.to_node = n.id
WHERE e.contract_hash != n.spec_hash;

-- 4. 특정 태그를 가진 모든 노드
SELECT n.* FROM nodes n
JOIN tags t ON n.id = t.node_id
WHERE t.tag = 'core';

-- 5. 레이어별 노드 통계
SELECT layer, COUNT(*) as count, 
       SUM(CASE WHEN status = 'implemented' THEN 1 ELSE 0 END) as implemented
FROM nodes
GROUP BY layer;

-- 6. 순환 참조 감지 (재귀 CTE)
WITH RECURSIVE dependency_chain(start_node, current_node, path, is_cycle) AS (
    -- 시작점
    SELECT id, id, id, 0
    FROM nodes
    
    UNION ALL
    
    -- 재귀 탐색
    SELECT 
        dc.start_node,
        e.to_node,
        dc.path || ' -> ' || e.to_node,
        CASE WHEN dc.path LIKE '%' || e.to_node || '%' THEN 1 ELSE 0 END
    FROM dependency_chain dc
    JOIN edges e ON dc.current_node = e.from_node
    WHERE dc.is_cycle = 0
)
SELECT DISTINCT start_node, path
FROM dependency_chain
WHERE is_cycle = 1;
```

---

## 5. CLI 명령어 규격 (CLI Commands)

### 5.1. 명령어 체계

```
gdc <command> [subcommand] [options] [arguments]
```

### 5.2. 전역 옵션

| 옵션 | 단축 | 설명 |
|------|------|------|
| `--config` | `-c` | 설정 파일 경로 지정 |
| `--verbose` | `-v` | 상세 출력 |
| `--quiet` | `-q` | 최소 출력 |
| `--json` | | JSON 형식 출력 |
| `--no-color` | | 색상 비활성화 |

---

### 5.3. 초기화 및 설정 (Initialization)

#### `gdc init`

프로젝트를 GDC로 초기화합니다.

```bash
gdc init [--language <lang>] [--storage <mode>]

# 예시
gdc init --language csharp --storage centralized
```

**동작:**
1. `.gdc/` 디렉토리 생성
2. `config.yaml` 생성 (대화형 또는 옵션 사용)
3. `graph.db` 초기화 (빈 스키마 생성)
4. `.gitignore`에 `graph.db` 추가 권장

**출력:**
```
✓ Created .gdc/config.yaml
✓ Initialized .gdc/graph.db
✓ Created .gdc/nodes/
ℹ Add '.gdc/graph.db' to your .gitignore
```

---

#### `gdc config`

설정 조회 및 수정.

```bash
gdc config get <key>
gdc config set <key> <value>
gdc config list

# 예시
gdc config set llm.default_model gpt-4-turbo
gdc config get project.language
```

---

### 5.4. 노드 관리 (Node Management)

#### `gdc node create`

새 노드 명세 파일을 생성합니다.

```bash
gdc node create <NodeName> [--type <type>] [--layer <layer>]

# 예시
gdc node create PlayerController --type class --layer application
gdc node create IInputManager --type interface
```

**옵션:**
| 옵션 | 기본값 | 설명 |
|------|--------|------|
| `--type` | `class` | `class`, `interface`, `module`, `service` |
| `--layer` | `application` | `domain`, `application`, `infrastructure`, `presentation` |
| `--template` | | 커스텀 템플릿 파일 |
| `--interactive` | | 대화형 모드 |

**생성되는 파일:** `.gdc/nodes/PlayerController.yaml`

---

#### `gdc node delete`

노드를 삭제합니다.

```bash
gdc node delete <NodeName> [--force]

# 예시
gdc node delete OldController --force
```

**동작:**
1. 해당 노드를 참조하는 엣지가 있는지 확인
2. `--force` 없이는 참조가 있으면 삭제 거부
3. YAML 파일 삭제 및 DB에서 제거

---

#### `gdc node rename`

노드 이름을 변경합니다.

```bash
gdc node rename <OldName> <NewName>

# 예시
gdc node rename PlayerController CharacterController
```

**동작:**
1. YAML 파일 이름 변경
2. 파일 내 `node.id` 수정
3. 모든 의존 엣지의 참조 업데이트
4. DB 동기화

---

### 5.5. 그래프 조회 (Graph Queries)

#### `gdc list`

모든 노드와 연결 상태를 요약 출력합니다.

```bash
gdc list [--filter <expr>] [--sort <field>] [--format <fmt>]

# 예시
gdc list
gdc list --filter "layer=domain"
gdc list --filter "status=draft" --sort updated_at
gdc list --format json
```

**출력 예시:**
```
┌─────────────────────┬───────────┬─────────────┬────────┬──────────┐
│ Node                │ Type      │ Layer       │ Status │ Deps     │
├─────────────────────┼───────────┼─────────────┼────────┼──────────┤
│ PlayerController    │ class     │ application │ ✓ impl │ 3 deps   │
│ IInputManager       │ interface │ application │ ✓ spec │ 0 deps   │
│ InputManager        │ class     │ infra       │ ○ draft│ 1 dep    │
│ IPhysicsEngine      │ interface │ domain      │ ✓ spec │ 0 deps   │
└─────────────────────┴───────────┴─────────────┴────────┴──────────┘
Total: 4 nodes | 4 edges
```

---

#### `gdc show`

특정 노드의 상세 정보를 표시합니다.

```bash
gdc show <NodeName> [--deps] [--refs] [--full]

# 예시
gdc show PlayerController
gdc show PlayerController --deps --refs
```

**옵션:**
| 옵션 | 설명 |
|------|------|
| `--deps` | 이 노드가 의존하는 노드들 표시 |
| `--refs` | 이 노드를 참조하는 노드들 표시 |
| `--full` | 전체 명세 출력 |
| `--interface-only` | 인터페이스 정보만 출력 |

**출력 예시:**
```
╔═══════════════════════════════════════════════════════════════╗
║  PlayerController                                              ║
╠═══════════════════════════════════════════════════════════════╣
║  Type: class | Layer: application | Status: implemented        ║
╟───────────────────────────────────────────────────────────────╢
║  Responsibility:                                               ║
║  플레이어 입력 처리 및 캐릭터 상태 관리                            ║
╟───────────────────────────────────────────────────────────────╢
║  Interface:                                                    ║
║  • Move(Vector2 direction): void                              ║
║  • Jump(): bool                                                ║
║  • TakeDamage(int amount, DamageType type): void              ║
║  • Health: int (get)                                          ║
║  • IsGrounded: bool (get)                                     ║
╟───────────────────────────────────────────────────────────────╢
║  Dependencies (→):                 │  Referenced by (←):       ║
║  • IInputManager [interface]       │  • GameManager            ║
║  • IPhysicsEngine [interface]      │  • CombatSystem           ║
║  • IAnimationController? [opt]     │                           ║
╚═══════════════════════════════════════════════════════════════╝
```

---

#### `gdc trace`

특정 노드로부터의 의존성 경로를 추적합니다.

```bash
gdc trace <NodeName> [--depth <n>] [--direction <dir>] [--to <target>]

# 예시
gdc trace PlayerController                    # 전체 의존성 트리
gdc trace PlayerController --depth 2          # 2단계까지만
gdc trace PlayerController --direction up     # 역방향 (이 노드를 참조하는 것들)
gdc trace PlayerController --to DatabaseService  # 특정 노드까지의 경로
```

**옵션:**
| 옵션 | 기본값 | 설명 |
|------|--------|------|
| `--depth` | 무제한 | 탐색 깊이 제한 |
| `--direction` | `down` | `down`(의존), `up`(역참조), `both` |
| `--to` | | 특정 노드까지의 경로만 표시 |

**출력 예시:**
```
PlayerController
├── IInputManager
│   └── (interface - no further deps)
├── IPhysicsEngine
│   └── (interface - no further deps)
└── IAnimationController [optional]
    └── (interface - no further deps)
```

---

#### `gdc graph`

그래프 데이터를 내보냅니다.

```bash
gdc graph [--format <fmt>] [--output <file>] [--filter <expr>]

# 예시
gdc graph --format dot --output architecture.dot
gdc graph --format json > graph.json
gdc graph --format mermaid --filter "layer=domain"
```

**지원 포맷:**
| 포맷 | 설명 |
|------|------|
| `dot` | Graphviz DOT 형식 |
| `json` | JSON 그래프 데이터 |
| `mermaid` | Mermaid 다이어그램 |
| `plantuml` | PlantUML 다이어그램 |

---

### 5.6. 동기화 및 검증 (Sync & Validation)

#### `gdc sync`

YAML 명세와 DB, 소스 코드를 동기화합니다.

```bash
gdc sync [--direction <dir>] [--dry-run] [--force]

# 예시
gdc sync                      # YAML → DB 동기화
gdc sync --direction code     # 소스 코드 → YAML 역동기화
gdc sync --dry-run            # 변경 사항만 표시
```

**옵션:**
| 옵션 | 설명 |
|------|------|
| `--direction` | `yaml`(기본), `code`(소스에서 추출) |
| `--dry-run` | 실제 변경 없이 미리보기 |
| `--force` | 충돌 시 강제 덮어쓰기 |

**동작 (yaml → db):**
1. YAML 파일들의 mtime 확인
2. 변경된 파일만 파싱
3. DB 업데이트 및 해시 재계산
4. 변경 로그 기록

**동작 (code → yaml):**
1. LSP 또는 AST 파서로 소스 분석
2. 공개 인터페이스 추출
3. 기존 YAML과 diff
4. 사용자 확인 후 YAML 업데이트

**출력:**
```
Scanning for changes...
  ⟳ Modified: PlayerController.yaml (spec_hash changed)
  ⟳ Modified: InputManager.yaml (new method detected)
  + Added: CombatSystem.yaml
  
Syncing 3 files...
  ✓ PlayerController: DB updated
  ✓ InputManager: DB updated, 1 new edge
  ✓ CombatSystem: Indexed

Sync complete: 3 updated, 0 errors
```

---

#### `gdc check`

그래프의 정합성을 검사합니다.

```bash
gdc check [--fix] [--category <cat>] [--severity <sev>]

# 예시
gdc check
gdc check --category hash_mismatch
gdc check --severity error --fix
```

**검사 항목:**

| 카테고리 | 설명 |
|----------|------|
| `missing_ref` | 존재하지 않는 노드를 참조 |
| `hash_mismatch` | 의존성 계약 해시 불일치 |
| `cycle` | 순환 참조 감지 |
| `orphan` | 고립된 노드 (아무도 참조하지 않음) |
| `srp_violation` | 단일 책임 원칙 위반 가능성 |
| `layer_violation` | 레이어드 아키텍처 위반 |
| `missing_impl` | 명세는 있지만 구현 파일 없음 |

**출력:**
```
Running validation checks...

[ERROR] Missing Reference
  └─ PlayerController → IAnimationController
     IAnimationController.yaml not found

[WARNING] Hash Mismatch  
  └─ GameManager → PlayerController
     Expected: a1b2c3d4, Current: x9y0z1a2
     PlayerController interface has changed since last sync

[WARNING] Potential SRP Violation
  └─ GameManager
     Node has 8 dependencies (threshold: 5)
     Consider splitting responsibilities

[INFO] Orphan Node
  └─ LegacyInputHandler
     Not referenced by any other node

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Summary: 1 error, 2 warnings, 1 info
```

---

#### `gdc watch`

파일 변경을 감시하고 자동 동기화합니다.

```bash
gdc watch [--check] [--notify]

# 예시
gdc watch                     # 동기화만
gdc watch --check             # 동기화 + 검증
```

**동작:**
1. `.gdc/nodes/` 및 소스 디렉토리 감시
2. 파일 변경 감지 시 `gdc sync` 실행
3. `--check` 옵션 시 `gdc check`도 실행

---

### 5.7. AI/LLM 프롬프트 생성 (Prompt Generation)

#### `gdc extract`

특정 노드를 구현하기 위한 최적화된 프롬프트를 생성합니다.

```bash
gdc extract <NodeName> [--template <name>] [--output <file>] [--depth <n>]

# 예시
gdc extract PlayerController
gdc extract PlayerController --template implement
gdc extract PlayerController --output prompt.md
```

**옵션:**
| 옵션 | 기본값 | 설명 |
|------|--------|------|
| `--template` | `implement` | 프롬프트 템플릿 선택 |
| `--output` | stdout | 파일로 저장 |
| `--depth` | 1 | 인접 노드 포함 깊이 |
| `--include-logic` | false | 내부 로직 스펙 포함 |
| `--clipboard` | false | 클립보드에 복사 |

**생성되는 프롬프트 예시:**
```markdown
# Implementation Request: PlayerController

## Target Node Specification

### Responsibility
플레이어 입력 처리 및 캐릭터 상태 관리
- 사용자 입력을 받아 캐릭터 행동으로 변환
- 이동, 점프, 공격 등 액션 상태 머신 관리
- 애니메이션 트리거 발생

### Public Interface to Implement

```csharp
public class PlayerController
{
    // Constructor
    public PlayerController(IInputManager input, IPhysicsEngine physics);
    
    // Methods
    public void Move(Vector2 direction);
    public bool Jump();
    public void TakeDamage(int amount, DamageType type);
    
    // Properties
    public int Health { get; }
    public bool IsGrounded { get; }
    
    // Events
    public event Action OnDeath;
    public event Action<int, DamageType> OnDamaged;
}
```

### Invariants
- 플레이어 속도는 항상 0 이상
- 체력은 0에서 maxHealth 사이

## Available Dependencies (Contracts)

### IInputManager
```csharp
interface IInputManager
{
    Vector2 GetMovementAxis();           // 정규화된 이동 축 벡터 반환
    bool IsActionPressed(string action); // 액션 버튼 상태 확인
    bool IsActionJustPressed(string action);
}
```

### IPhysicsEngine
```csharp
interface IPhysicsEngine
{
    void ApplyForce(Vector3 force);
    RaycastHit? Raycast(Vector3 origin, Vector3 direction);
}
```

### IAnimationController (Optional)
```csharp
interface IAnimationController
{
    void SetTrigger(string name);
    void SetFloat(string name, float value);
}
```

## Implementation Notes
- Use the provided interfaces ONLY. Do not access any other systems.
- The state machine should start from "Idle" state.
- Jump is only allowed when IsGrounded is true.
```

---

#### `gdc prompt`

LLM을 위한 다양한 프롬프트를 생성합니다.

```bash
gdc prompt <type> [options]

# 타입별 예시
gdc prompt implement PlayerController          # 구현 요청
gdc prompt review PlayerController             # 코드 리뷰 요청
gdc prompt refactor PlayerController           # 리팩토링 제안 요청
gdc prompt test PlayerController               # 테스트 코드 생성 요청
gdc prompt explain PlayerController            # 설계 의도 설명 요청
gdc prompt impact IInputManager                # 변경 영향도 분석 요청
```

---

#### `gdc validate-design`

LLM을 통해 설계의 논리적 정합성을 검증합니다.

```bash
gdc validate-design [--scope <scope>] [--focus <focus>]

# 예시
gdc validate-design                            # 전체 검증
gdc validate-design --scope PlayerController   # 특정 노드 중심
gdc validate-design --focus srp               # SRP에 집중
```

**검증 항목:**
- 순환 참조 여부
- 단일 책임 원칙(SRP) 준수
- 레이어드 아키텍처 위반
- 불필요한 의존성
- 인터페이스 분리 원칙(ISP) 위반
- 명명 규칙 일관성

**출력:**
```
🔍 Design Validation Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ No circular dependencies detected
✅ Layer architecture: All dependencies flow correctly

⚠️  SRP Concern: GameManager
   └─ This class has 8 dependencies and 12 methods.
   └─ Suggestion: Consider extracting GameStateManager and EventDispatcher

⚠️  Interface Segregation: IGameService
   └─ This interface has 15 methods.
   └─ Suggestion: Split into IGameLifecycle, IGameState, IGameEvents

💡 Naming Inconsistency:
   └─ Some classes use "Manager" suffix, others use "Controller"
   └─ Recommendation: Standardize on one pattern

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Overall Score: 7.5/10
```

---

### 5.8. 통계 및 리포트 (Statistics & Reports)

#### `gdc stats`

프로젝트 통계를 출력합니다.

```bash
gdc stats [--format <fmt>]

# 예시
gdc stats
gdc stats --format json
```

**출력:**
```
📊 Project Statistics
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Nodes: 24 total
  ├─ Classes:    15 (62.5%)
  ├─ Interfaces: 8  (33.3%)
  └─ Services:   1  (4.2%)

By Layer:
  ├─ Domain:         5
  ├─ Application:    10
  ├─ Infrastructure: 6
  └─ Presentation:   3

By Status:
  ├─ Implemented: 18 (75%)
  ├─ Specified:   4  (16.7%)
  └─ Draft:       2  (8.3%)

Edges: 42 total
  ├─ Interface deps: 35
  └─ Class deps:     7

Coverage:
  └─ Nodes with specs: 24/24 (100%)
  └─ Nodes with impl:  18/24 (75%)

Health:
  ├─ Validation errors:   0
  ├─ Hash mismatches:     2
  └─ Orphan nodes:        1
```

---

#### `gdc report`

상세 리포트를 생성합니다.

```bash
gdc report [--type <type>] [--output <file>]

# 예시
gdc report --type architecture --output report.md
gdc report --type dependencies --output deps.html
```

**리포트 타입:**
| 타입 | 설명 |
|------|------|
| `architecture` | 전체 아키텍처 개요 마크다운 |
| `dependencies` | 의존성 매트릭스 |
| `coverage` | 명세/구현 커버리지 |
| `changelog` | 최근 변경 이력 |

---

## 6. 프롬프트 템플릿 시스템 (Template System)

### 6.1. 템플릿 위치

`.gdc/templates/` 디렉토리에 Jinja2 형식으로 저장

### 6.2. 내장 템플릿

#### `implement.md.j2`

```jinja2
{# Implementation Request Template #}
# Implementation Request: {{ node.id }}

## Target Node Specification

### Responsibility
{{ node.responsibility.summary }}
{% if node.responsibility.details %}
{{ node.responsibility.details }}
{% endif %}

### Public Interface to Implement

```{{ config.language }}
{% if node.type == 'class' %}
public class {{ node.id }}
{% elif node.type == 'interface' %}
public interface {{ node.id }}
{% endif %}
{
{% for ctor in node.interface.constructors %}
    {{ ctor.signature }};
{% endfor %}

{% for method in node.interface.methods %}
    {{ method.signature }};  // {{ method.description }}
{% endfor %}

{% for prop in node.interface.properties %}
    {{ prop.type }} {{ prop.name }} { {{ prop.access }}; }
{% endfor %}

{% for event in node.interface.events %}
    {{ event.signature }};
{% endfor %}
}
```

{% if node.responsibility.invariants %}
### Invariants
{% for inv in node.responsibility.invariants %}
- {{ inv }}
{% endfor %}
{% endif %}

## Available Dependencies (Contracts)

{% for dep in dependencies %}
### {{ dep.target }}{% if dep.optional %} (Optional){% endif %}

```{{ config.language }}
{{ dep.interface_code }}
```

Usage:
{{ dep.usage }}

{% endfor %}

## Implementation Notes
- Use the provided interfaces ONLY. Do not access any other systems.
{% if node.logic and node.logic.state_machine %}
- Implement the following state machine:
  - Initial state: {{ node.logic.state_machine.initial }}
  - States: {{ node.logic.state_machine.states | map(attribute='name') | join(', ') }}
{% endif %}
```

#### `review.md.j2`

```jinja2
{# Code Review Request Template #}
# Code Review Request: {{ node.id }}

## Specification

{{ node.responsibility.summary }}

### Expected Interface
```{{ config.language }}
{{ node.interface_code }}
```

### Dependencies Used
{% for dep in dependencies %}
- {{ dep.target }}: {{ dep.usage | first }}
{% endfor %}

## Current Implementation

```{{ config.language }}
{{ implementation_code }}
```

## Review Focus
1. Does the implementation match the specification?
2. Are all invariants maintained?
3. Is error handling appropriate?
4. Are the dependencies used correctly per their contracts?

Please provide:
- Conformance score (0-100)
- Issues found
- Improvement suggestions
```

---

## 7. 해시 계산 규칙 (Hash Calculation)

### 7.1. Spec Hash

노드의 **공개 인터페이스 변경**을 감지하기 위한 해시.

**포함 항목:**
- `interface.methods[*].signature`
- `interface.properties[*].type` + `access`
- `interface.events[*].signature`
- `interface.constructors[*].signature`

**제외 항목:**
- `description`, `usage` 등 문서적 내용
- `metadata`
- `logic` (내부 구현 상세)

**계산:**
```python
def calculate_spec_hash(node):
    parts = []
    for method in node.interface.methods:
        parts.append(method.signature)
    for prop in node.interface.properties:
        parts.append(f"{prop.name}:{prop.type}:{prop.access}")
    for event in node.interface.events:
        parts.append(event.signature)
    
    combined = "|".join(sorted(parts))
    return sha256(combined)[:8]
```

### 7.2. Impl Hash

소스 코드 파일의 변경을 감지하기 위한 해시.

**계산:**
```python
def calculate_impl_hash(file_path):
    content = read_file(file_path)
    # 공백, 주석 정규화 후 해시
    normalized = normalize_code(content)
    return sha256(normalized)[:8]
```

### 7.3. Contract Hash

의존 시점의 대상 노드 인터페이스 해시.

```python
# A가 B를 의존할 때
edge.contract_hash = B.spec_hash  # 의존 선언 시점의 B 해시
```

**불일치 감지:**
```python
if edge.contract_hash != target_node.spec_hash:
    # B의 인터페이스가 변경됨
    # A의 구현을 재검토해야 함
```

---

## 8. 에러 코드 (Error Codes)

| 코드 | 설명 |
|------|------|
| `GDC001` | 초기화되지 않은 프로젝트 |
| `GDC002` | 노드를 찾을 수 없음 |
| `GDC003` | YAML 파싱 에러 |
| `GDC004` | 스키마 유효성 검사 실패 |
| `GDC005` | 순환 참조 감지 |
| `GDC006` | 참조 무결성 위반 |
| `GDC007` | 해시 불일치 |
| `GDC008` | DB 접근 에러 |
| `GDC009` | 파일 시스템 에러 |
| `GDC010` | 템플릿 렌더링 에러 |

---

## 9. 환경 변수 (Environment Variables)

| 변수 | 설명 |
|------|------|
| `GDC_HOME` | GDC 전역 설정 디렉토리 |
| `GDC_CONFIG` | 설정 파일 경로 오버라이드 |
| `GDC_DB_PATH` | DB 경로 오버라이드 |
| `GDC_LOG_LEVEL` | 로그 레벨 (debug, info, warn, error) |
| `GDC_NO_COLOR` | 색상 출력 비활성화 |
| `GDC_LLM_API_KEY` | LLM API 키 |
| `GDC_LLM_MODEL` | 기본 LLM 모델 |

---

## 10. 로드맵 (Roadmap)

### Phase 1: Core (MVP)
- [x] 프로젝트 초기화 (`init`)
- [x] 노드 CRUD (`node create/delete/rename`)
- [x] YAML-DB 동기화 (`sync`)
- [x] 기본 조회 (`list`, `show`)
- [ ] 기본 검증 (`check`)

### Phase 2: Graph Intelligence
- [ ] 의존성 추적 (`trace`)
- [ ] 그래프 내보내기 (`graph`)
- [ ] 순환 참조 감지
- [ ] 레이어 위반 감지

### Phase 3: AI Integration
- [ ] 프롬프트 추출 (`extract`)
- [ ] 다양한 프롬프트 템플릿
- [ ] LLM 설계 검증 (`validate-design`)

### Phase 4: Advanced
- [ ] 파일 감시 (`watch`)
- [ ] 소스 코드 역동기화
- [ ] LSP 통합
- [ ] VS Code 확장

---

## Appendix A: 전체 CLI 명령어 요약

```
gdc
├── init                    # 프로젝트 초기화
├── config                  # 설정 관리
│   ├── get <key>
│   ├── set <key> <value>
│   └── list
├── node                    # 노드 관리
│   ├── create <name>
│   ├── delete <name>
│   └── rename <old> <new>
├── list                    # 전체 노드 목록
├── show <node>             # 노드 상세 정보
├── trace <node>            # 의존성 경로 추적
├── graph                   # 그래프 내보내기
├── sync                    # YAML-DB 동기화
├── check                   # 정합성 검사
├── watch                   # 파일 감시
├── extract <node>          # AI 프롬프트 추출
├── prompt <type> <node>    # 다양한 프롬프트 생성
├── validate-design         # LLM 설계 검증
├── stats                   # 통계
└── report                  # 상세 리포트
```

---

## Appendix B: 용어 사전 (Glossary)

| 용어 | 정의 |
|------|------|
| **Node** | 시스템의 독립적 구현 단위 (클래스, 인터페이스, 모듈) |
| **Edge** | 노드 간의 의존성 관계 |
| **Spec** | YAML로 작성된 노드의 명세서 |
| **Contract** | 노드가 제공하는 공개 인터페이스의 약속 |
| **Spec Hash** | 인터페이스 변경을 감지하기 위한 해시 |
| **Impl Hash** | 구현 코드 변경을 감지하기 위한 해시 |
| **Contract Hash** | 의존 시점의 대상 인터페이스 해시 |
| **SDD** | Specification-Driven Development |
| **Context Isolation** | AI에게 최소 컨텍스트만 제공하는 전략 |

---

*End of Specification*
