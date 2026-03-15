---
name: gdc-node-spec
description: Write GDC node specification YAML files. Use when creating or editing node specs for classes, interfaces, services in GDC projects.
license: MIT
compatibility: opencode
metadata:
  category: specification
  language: yaml
  workflow: sdd
---

# GDC Node Specification Writing Guide

GDC 노드 명세 YAML 파일 작성을 위한 가이드입니다.

## When to Use This Skill

- 새로운 노드 명세 파일 작성 시
- 기존 명세 수정 또는 보완 시
- 인터페이스 시그니처 정의 시
- 의존성 관계 설정 시

## Complete Node Schema

```yaml
schema_version: "1.0"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 노드 기본 정보
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
node:
  id: "NodeName"                    # 필수: 노드 식별자
  type: "class"                     # 필수: class | interface | service | module | enum
  layer: "application"              # 필수: domain | application | infrastructure | presentation
  namespace: "MyApp.Services"       # 선택: 네임스페이스
  file_path: "internal/service.go"  # 선택: 구현 파일 경로

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 책임 정의
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
responsibility:
  summary: "한 문장으로 이 노드의 핵심 책임을 설명"  # 필수
  
  details: |                        # 선택: 상세 설명
    더 자세한 설명이 필요한 경우 여기에 작성합니다.
    여러 줄로 작성할 수 있습니다.
  
  invariants:                       # 선택: 불변 조건
    - "모든 public 메서드 호출 후에도 유지되어야 하는 조건"
    - "객체 상태에 대한 보장 사항"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 인터페이스 정의
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
interface:
  # 생성자 (클래스 타입에서 사용)
  constructors:
    - signature: "NewNodeName(repo IRepository) *NodeName"
      description: "의존성을 주입받아 인스턴스 생성"
      parameters:
        - name: "repo"
          type: "IRepository"
          description: "데이터 접근 인터페이스"

  # 메서드
  methods:
    - name: "Execute"
      signature: "Execute(ctx context.Context, input InputDTO) (OutputDTO, error)"
      description: "주요 비즈니스 로직 실행"
      parameters:
        - name: "ctx"
          type: "context.Context"
          description: "요청 컨텍스트"
          constraint: "nil이 아니어야 함"
        - name: "input"
          type: "InputDTO"
          description: "입력 데이터"
      returns:
        type: "OutputDTO, error"
        description: "처리 결과 또는 에러"
      throws:
        - type: "ErrNotFound"
          condition: "리소스를 찾을 수 없는 경우"
        - type: "ErrValidation"
          condition: "입력 데이터가 유효하지 않은 경우"

    - name: "GetStatus"
      signature: "GetStatus() Status"
      description: "현재 상태 조회"
      returns:
        type: "Status"
        description: "현재 상태 열거값"

  # 속성
  properties:
    - name: "ID"
      type: "string"
      access: "get"                 # get | set | get set
      description: "고유 식별자"
      
    - name: "Name"
      type: "string"
      access: "get set"
      description: "이름 (변경 가능)"

  # 이벤트 (C# 등에서 사용)
  events:
    - name: "OnStatusChanged"
      signature: "event EventHandler<StatusChangedEventArgs> OnStatusChanged"
      description: "상태 변경 시 발생"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 의존성 정의
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
dependencies:
  - target: "IRepository"           # 필수: 의존 대상 노드 ID
    type: "interface"               # 필수: interface | class | service
    injection: "constructor"        # 필수: constructor | property | method | factory
    optional: false                 # 선택: 선택적 의존성 여부
    contract_hash: "a1b2c3d4"       # 선택: 계약 해시 (변경 감지용)
    usage: |                        # 선택: 사용 방법 설명
      - FindByID(): ID로 엔티티 조회
      - Save(): 엔티티 저장
      - Delete(): 엔티티 삭제

  - target: "ILogger"
    type: "interface"
    injection: "constructor"
    optional: true                  # 로깅은 선택적
    usage: |
      - Info(): 정보 로깅
      - Error(): 에러 로깅

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 내부 로직 (선택)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
logic:
  state_machine:
    initial: "Idle"
    states:
      - name: "Idle"
        description: "대기 상태"
        transitions:
          - to: "Processing"
            trigger: "Execute() 호출"
      - name: "Processing"
        description: "처리 중"
        transitions:
          - to: "Completed"
            trigger: "처리 완료"
          - to: "Failed"
            trigger: "에러 발생"
      - name: "Completed"
        description: "완료"
      - name: "Failed"
        description: "실패"

  algorithms:
    - name: "ValidationAlgorithm"
      description: |
        입력 데이터 검증 알고리즘:
        1. 필수 필드 존재 확인
        2. 형식 검증
        3. 비즈니스 규칙 검증

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 구현 정보 (인터페이스 타입에서 사용)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
implementations:
  - "ConcreteImplementation1"
  - "ConcreteImplementation2"

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 메타데이터
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
metadata:
  status: "specified"               # draft | specified | implemented | tested | deprecated
  created: "2026-02-03"
  updated: "2026-02-03"
  tags: ["core", "domain", "service"]
  notes: |                          # 선택: 추가 메모
    구현 시 주의사항이나 특별한 고려사항
  spec_hash: ""                     # 자동 생성: 명세 해시
  impl_hash: ""                     # 자동 생성: 구현 해시
```

## Language-Specific Examples

### Go Interface

```yaml
schema_version: "1.0"

node:
  id: "IUserRepository"
  type: "interface"
  layer: "domain"
  namespace: "domain/repository"

responsibility:
  summary: "사용자 엔티티의 영속성을 관리하는 저장소 인터페이스"

interface:
  methods:
    - name: "FindByID"
      signature: "FindByID(ctx context.Context, id string) (*User, error)"
      description: "ID로 사용자 조회"
    - name: "FindByEmail"
      signature: "FindByEmail(ctx context.Context, email string) (*User, error)"
      description: "이메일로 사용자 조회"
    - name: "Save"
      signature: "Save(ctx context.Context, user *User) error"
      description: "사용자 저장 (생성/수정)"
    - name: "Delete"
      signature: "Delete(ctx context.Context, id string) error"
      description: "사용자 삭제"

implementations:
  - "PostgresUserRepository"
  - "InMemoryUserRepository"

metadata:
  status: "specified"
  tags: ["domain", "repository"]
```

### Go Struct (Class)

```yaml
schema_version: "1.0"

node:
  id: "UserService"
  type: "class"
  layer: "application"
  namespace: "application/service"
  file_path: "internal/application/user_service.go"

responsibility:
  summary: "사용자 관련 비즈니스 로직을 처리하는 애플리케이션 서비스"
  invariants:
    - "모든 사용자 데이터는 Repository를 통해서만 접근"

interface:
  constructors:
    - signature: "NewUserService(repo IUserRepository, logger ILogger) *UserService"
      description: "의존성 주입을 통한 서비스 생성"

  methods:
    - name: "RegisterUser"
      signature: "RegisterUser(ctx context.Context, req RegisterRequest) (*User, error)"
      description: "새 사용자 등록"
    - name: "GetUser"
      signature: "GetUser(ctx context.Context, id string) (*User, error)"
      description: "사용자 정보 조회"

dependencies:
  - target: "IUserRepository"
    type: "interface"
    injection: "constructor"
    usage: |
      - FindByID(): 사용자 조회
      - Save(): 사용자 저장
  - target: "ILogger"
    type: "interface"
    injection: "constructor"
    optional: true

metadata:
  status: "specified"
  tags: ["application", "service"]
```

### TypeScript Interface

```yaml
schema_version: "1.0"

node:
  id: "IUserService"
  type: "interface"
  layer: "application"
  namespace: "services"

responsibility:
  summary: "사용자 관련 비즈니스 로직 인터페이스"

interface:
  methods:
    - name: "getUser"
      signature: "getUser(id: string): Promise<User | null>"
      description: "ID로 사용자 조회"
    - name: "createUser"
      signature: "createUser(data: CreateUserDTO): Promise<User>"
      description: "새 사용자 생성"
    - name: "updateUser"
      signature: "updateUser(id: string, data: UpdateUserDTO): Promise<User>"
      description: "사용자 정보 수정"

  properties:
    - name: "isInitialized"
      type: "boolean"
      access: "get"
      description: "서비스 초기화 여부"

metadata:
  status: "specified"
  tags: ["service", "user"]
```

### C# Interface

```yaml
schema_version: "1.0"

node:
  id: "IGameService"
  type: "interface"
  layer: "application"
  namespace: "GameApp.Services"

responsibility:
  summary: "게임 로직을 처리하는 서비스 인터페이스"

interface:
  methods:
    - name: "StartGame"
      signature: "Task<GameSession> StartGameAsync(GameConfig config)"
      description: "새 게임 세션 시작"
    - name: "EndGame"
      signature: "Task EndGameAsync(Guid sessionId)"
      description: "게임 세션 종료"

  properties:
    - name: "ActiveSessions"
      type: "IReadOnlyList<GameSession>"
      access: "get"
      description: "현재 활성 세션 목록"

  events:
    - name: "OnGameStarted"
      signature: "event EventHandler<GameStartedEventArgs> OnGameStarted"
      description: "게임 시작 시 발생"
    - name: "OnGameEnded"
      signature: "event EventHandler<GameEndedEventArgs> OnGameEnded"
      description: "게임 종료 시 발생"

metadata:
  status: "specified"
  tags: ["game", "service"]
```

## Quick Templates

### Minimal Interface

```yaml
schema_version: "1.0"
node:
  id: "IMyInterface"
  type: "interface"
  layer: "domain"
responsibility:
  summary: "인터페이스 책임 설명"
interface:
  methods:
    - name: "DoSomething"
      signature: "DoSomething() error"
      description: "메서드 설명"
metadata:
  status: "draft"
```

### Minimal Class

```yaml
schema_version: "1.0"
node:
  id: "MyClass"
  type: "class"
  layer: "application"
responsibility:
  summary: "클래스 책임 설명"
interface:
  constructors:
    - signature: "NewMyClass(dep IDependency) *MyClass"
      description: "생성자"
  methods:
    - name: "Execute"
      signature: "Execute() error"
      description: "메서드 설명"
dependencies:
  - target: "IDependency"
    type: "interface"
    injection: "constructor"
metadata:
  status: "draft"
```

## Checklist for Good Specs

- [ ] `node.id`가 파일 이름과 일치하는가?
- [ ] `responsibility.summary`가 명확하고 간결한가?
- [ ] 모든 메서드에 `description`이 있는가?
- [ ] 모든 `dependencies`가 실제 존재하는 노드인가?
- [ ] `layer`가 아키텍처 규칙을 따르는가?
- [ ] 불변 조건(`invariants`)이 명확한가?
