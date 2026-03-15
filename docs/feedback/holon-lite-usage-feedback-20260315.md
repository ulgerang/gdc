# GDC 현장 사용 피드백

**프로젝트**: holon-lite (`github.com/ulgerang/holon-lite`)  
**작성일**: 2026-03-15  
**작성자**: Antigravity (AI 코딩 어시스턴트)  
**사용 맥락**: 대형 Go 프로젝트(~714 노드) 5-Phase 아키텍처 재설계 작업 중 GDC를 설계-추적 도구로 활용

---

## 요약

GDC는 대규모 Go 프로젝트에서 **"무엇이 어디에 있는가"를 추적하고 구현 컨텍스트를 조립하는 데** 실질적인 도움이 됐습니다.  
특히 `gdc show --deps --refs`, `gdc extract --with-impl`, `gdc query` 는 코드 리뷰와 구현 컨텍스트 파악에 유용했습니다.

아래는 활용도를 더 높이기 위해 현장에서 느낀 구체적인 피드백입니다.

---

## 피드백 1: `status: draft`의 의미가 너무 모호합니다

### 현상

`gdc sync --direction code`로 자동 추출된 노드도, 수동으로 작성한 설계 노드도 모두 기본 status가 `draft`로 설정됩니다.

이 때문에 다음 두 가지를 구별할 수 없습니다:

- **"설계됐지만 아직 구현되지 않은 노드"** → 진짜 draft, 할 일이 있음
- **"코드에서 자동 추출됐는데 아무도 status를 바꾸지 않은 노드"** → 사실상 stable, 이미 구현됨

결과적으로 "draft 노드 중 구현이 필요한 게 있는가?"라는 질문에 GDC가 답을 줄 수 없었습니다.

### 제안

노드의 출처(provenance)를 metadata에 추가해 주세요.

```yaml
metadata:
  status: draft
  origin: code_extracted   # vs. hand_authored
  extracted_at: 2026-03-15
```

- `origin: code_extracted`인 노드는 자동으로 `stable`로 표시하거나, sync 시에 status를 `stable`로 승격하는 옵션을 제공
- `origin: hand_authored`인 노드만 "구현 검증 대상"으로 필터링 가능하게
- `gdc list --hand-authored --status draft` 같은 필터 쿼리 지원

---

## 피드백 2: `gdc sync --direction code`가 의존관계(deps 엣지)를 추출하지 않습니다

### 현상

이번 프로젝트에서 노드 714개 중 637개(89%)가 orphan이었습니다.

```
Summary: 0 errors, 0 warnings, 637 info
```

고아의 대부분은 **Go import 관계와 struct 필드 타입이 GDC 그래프에 반영되지 않아서** 발생합니다.  
예를 들어 `Agent`는 코드에서 `LLM`, `Config`, `Session`, `RunContext`를 명백히 사용하지만, GDC 그래프에는 아무 연결도 없습니다.

결과적으로 `gdc trace Agent --reverse` 같은 blast radius 분석이 빈 결과를 반환해 실용성이 없었습니다.

### 제안

`gdc sync --direction code` 실행 시, Go 소스 분석을 통해 `dependencies` 섹션을 자동으로 채워주세요.

```yaml
# 현재 (수동 작성 없으면 빈 상태)
dependencies: []

# 제안 (auto-extracted from code)
dependencies:
  - LLM          # Agent struct 필드 타입
  - Config       # Agent struct 필드 타입
  - RunContext   # CloneForRun 파라미터 타입
  - Session      # Agent struct 필드 타입
```

분석 소스 후보:

1. **struct 필드 타입** → 가장 직접적인 의존관계
2. **함수 파라미터/반환 타입**
3. **패키지 import** → 패키지 레벨 의존관계

이것만 구현돼도 고아 637개 → 수십 개로 줄고,  
`gdc trace`가 실제 리팩토링 의사결정에 유용한 도구가 됩니다.

---

## 피드백 3: `function` 타입이 DB 스키마에서 허용되지 않습니다

### 현상

`ValidateConfigOwnershipMatrix`처럼 **패키지 레벨 독립 함수(package-level function)** 를 GDC 노드로 등록하려 했을 때, sync 실패가 발생했습니다.

```
Failed to sync ValidateConfigOwnershipMatrix:
constraint failed: CHECK constraint failed: type IN ('class', 'interface', 'module', 'service', 'enum')
```

YAML에 `type: function`으로 작성했지만 DB 스키마가 이를 허용하지 않습니다.

### 제안

허용 타입에 `function`을 추가해 주세요.

```sql
-- 현재
type IN ('class', 'interface', 'module', 'service', 'enum')

-- 제안
type IN ('class', 'interface', 'module', 'service', 'enum', 'function')
```

Go, Python, Rust, Kotlin 등 함수형 스타일이 혼합된 언어에서 패키지 레벨 함수는 매우 흔합니다. 특히 Go에서는 생성자 함수(`New*`), 팩토리 함수, 유틸리티 함수가 타입에 종속되지 않고 패키지 레벨에 독립적으로 존재하는 것이 관용적입니다.

---

## 피드백 4: 설계 노드의 구현 검증이 없습니다

### 현상

GDC를 설계-우선(design-first) 방식으로 쓸 때 가장 필요한 것은:

> **"이 노드의 YAML에 명시된 인터페이스가 실제 코드에 구현되어 있는가?"**

현재 `file_path`가 있지만 `gdc check`는 해당 파일에서 노드에 대응하는 심볼이 실제로 존재하는지 검증하지 않습니다.
즉, `file_path`가 존재하지 않는 파일을 가리켜도 경고가 없습니다.

### 제안

`gdc check --verify-impl` 옵션 추가:

```
$ gdc check --verify-impl

[ERROR] SomeDesignNode: file_path exists but no struct 'SomeDesignNode' found
         in pkg/agent/design_node.go
[WARN]  DelegationPolicy: 4/5 methods matched (missing: EvaluateCapability)
[OK]    DelegationRequest: struct found, all 3 methods matched
```

이 기능이 있으면:

- `status: draft` + `origin: hand_authored` + `impl: missing` → **진짜로 구현이 필요한 노드** 자동 식별
- 리팩토링 후 YAML 스펙과 코드의 drift 감지 가능
- CI에서 `gdc check --verify-impl --fail-on-missing`으로 설계-구현 정합성 게이트 구축 가능

---

## 피드백 5: YAML 스펙과 실제 코드 간 drift 감지가 없습니다

### 현상

YAML에 메서드 시그니처를 적어두고 이후 코드에서 리팩토링하면 GDC는 이를 알지 못합니다.  
특히 `gdc sync --direction code`를 실행해도 기존 수동 작성 YAML을 덮어쓰지 않아 스펙이 코드 현실과 점점 어긋납니다.

### 제안

```
$ gdc diff Agent

  Method 'RegisterTools'
    YAML spec:  RegisterTools(tools unknown) error
    Code actual: RegisterTools(tools []Tool) error   ← type mismatch

  Property 'Garden'
    YAML spec:  type: ContextInjector
    Code actual: type: *garden.Gardener             ← type changed
```

YAML을 코드 현실에 맞게 자동 업데이트하는 `gdc sync --merge` 모드도 유용할 것입니다:

```
$ gdc sync --direction code --merge
# 수동 작성 description/responsibility는 보존
# 코드에서 변경된 signature/type만 업데이트
```

---

## 피드백 6: `gdc extract --with-callers`가 deps 엣지 없이 작동하지 않습니다

### 현상

`gdc extract NodeName --with-impl --with-callers` 사용 시,  
deps 엣지가 없는 현재 상태에서는 `--with-callers` 섹션이 대부분 비어서 반환됩니다.

caller 추적이 의존관계 엣지에 의존하기 때문입니다.

### 제안

엣지가 없을 때 **코드 기반 역참조 검색(grep 폴백)** 을 옵션으로 제공해 주세요.

```
$ gdc extract DelegationRequest --with-callers --grep-fallback

  Callers (from dependency graph):
    (none - no edges defined)

  Callers (from code search - fallback):
    pkg/agent/task_tool.go:283       BuildDelegationRequest(ctx, req, depth)
    pkg/agent/task_tool_execute.go:41  req, err := BuildDelegationRequest(...)
    pkg/agent/delegation_contract_test.go:12  req := DelegationRequest{...}
```

"엣지 기반 callers"와 "grep 기반 callers"를 구분해서 보여주면 신뢰도도 유지됩니다.

---

## 정리: 우선순위

| 번호 | 피드백 | 체감 임팩트 | 구현 난이도 |
|------|--------|-----------|-----------|
| 2 | deps 자동 추출 (struct 필드/파라미터 기반) | ★★★★★ | 높음 |
| 4 | 설계 노드 구현 검증 (`--verify-impl`) | ★★★★★ | 중간 |
| 1 | `origin: code_extracted` provenance 필드 | ★★★★☆ | 낮음 |
| 5 | YAML-코드 drift 감지 (`gdc diff`) | ★★★★☆ | 높음 |
| 3 | `function` 타입 DB 허용 | ★★★☆☆ | 낮음 |
| 6 | `--with-callers` grep 폴백 | ★★★☆☆ | 중간 |

**2번(deps 자동 추출)과 4번(구현 검증)** 이 두 개만 구현돼도  
GDC가 "타입 문서화 도구"에서 "살아있는 설계-구현 정합성 도구"로 도약할 수 있다고 생각합니다.

---

## GDC가 잘 작동한 부분

비판만 하는 건 아닙니다. 실제로 유용했던 것들:

- **`gdc query <name>`**: 부분 이름이나 namespace로 빠르게 노드 찾기. 714개 노드에서 정확하게 동작했습니다.
- **`gdc show --full`**: 노드의 인터페이스와 responsibility를 한눈에 확인. 구현 전 설계 리뷰에 유용했습니다.
- **`gdc extract --with-impl`**: 구현 컨텍스트와 스펙을 함께 추출해 AI 코딩 어시스턴트의 컨텍스트 조립에 직접 활용 가능.
- **`gdc check`**: 0 errors 확인이 빠르고 명확. CI 게이트로 쓰기 좋습니다.
- **노드 YAML의 `responsibility.summary`**: 코드 코멘트와 별도로 "왜 이 타입이 존재하는가"를 기록할 수 있는 공간이 매우 유용했습니다.

감사합니다.
