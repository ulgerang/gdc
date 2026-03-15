---
description: 노드의 의존성 및 영향 범위 분석하기
---

# Trace Dependencies Workflow

특정 노드를 수정할 때 영향받는 다른 노드들을 분석하는 워크플로우입니다.

## Prerequisites
- GDC가 초기화된 프로젝트
- DB가 동기화된 상태 (`gdc sync` 완료)

## Steps

// turbo
1. **노드 정보 확인**: 대상 노드의 기본 정보를 확인합니다.
   ```
   gdc show <NodeName>
   ```

// turbo
2. **의존성 추적 (downstream)**: 이 노드가 의존하는 노드들을 확인합니다.
   ```
   gdc trace <NodeName>
   ```

// turbo
3. **역의존성 추적 (upstream)**: 이 노드에 의존하는 노드들을 확인합니다.
   ```
   gdc trace <NodeName> --reverse
   ```

4. **영향 분석**: 추적 결과를 바탕으로 수정 영향도를 분석합니다.
   - 인터페이스 시그니처 변경 시: 모든 구현체와 사용처에 영향
   - 클래스 내부 로직 변경 시: 직접적인 영향 없음
   - 의존성 추가/제거 시: 생성자/팩토리 코드 수정 필요

5. **관련 명세 수집**: 수정에 필요한 모든 관련 YAML을 확인합니다.
   ```
   gdc extract <NodeName>
   ```

## Example Usage

```
/trace-deps IUserRepository
```

## Output Interpretation

```
IUserRepository (interface)
├── ← UserService (uses)        # UserService가 이 인터페이스 사용
├── ← OrderService (uses)       # OrderService도 사용
└── → PostgresUserRepository    # 이 인터페이스의 구현체
```

## Notes
- `--reverse` 옵션으로 "누가 나를 사용하는가" 확인
- `--depth N` 옵션으로 추적 깊이 제한 가능
- 그래프 시각화: `gdc graph --format dot` 후 Graphviz 사용
