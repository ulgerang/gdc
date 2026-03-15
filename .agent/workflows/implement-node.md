---
description: YAML 명세 기반으로 노드(클래스/인터페이스/서비스) 구현하기
---

# Node Implementation Workflow

YAML 명세를 기반으로 코드를 구현하는 워크플로우입니다.

## Prerequisites
- GDC가 초기화된 프로젝트 (`gdc init` 완료)
- 구현할 노드의 YAML 명세가 `.gdc/nodes/` 디렉토리에 존재

## Steps

1. **노드 명세 확인**: 사용자가 요청한 노드의 YAML 명세 파일을 읽습니다.
   - 파일 위치: `.gdc/nodes/<NodeName>.yaml`
   - `gdc show <NodeName>` 명령으로도 확인 가능

// turbo
2. **의존성 추적**: 해당 노드가 의존하는 다른 노드들을 확인합니다.
   ```
   gdc trace <NodeName>
   ```

// turbo
3. **구현 프롬프트 추출**: AI 구현에 필요한 컨텍스트를 수집합니다.
   ```
   gdc extract <NodeName>
   ```

4. **코드 구현**: 수집된 명세와 의존성 정보를 바탕으로 코드를 작성합니다.
   - YAML의 `interface.methods`에 정의된 시그니처를 정확히 구현
   - YAML의 `dependencies`에 명시된 의존성을 생성자/메서드로 주입
   - YAML의 `responsibility.summary`를 주석으로 포함

5. **명세 동기화**: 구현 후 YAML 명세를 업데이트합니다.
   ```
   gdc sync --direction yaml
   ```

// turbo
6. **정합성 검증**: 코드와 명세 간의 불일치가 없는지 확인합니다.
   ```
   gdc check
   ```

## Example Usage

```
/implement-node UserService
```

## Notes
- 구현 시 `metadata.status`를 `specified` → `implemented`로 변경
- 새로운 메서드 추가 시 YAML도 함께 업데이트
