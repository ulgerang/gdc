---
description: 코드 변경 후 YAML 명세 자동 업데이트하기
---

# Update Specification Workflow

코드를 수정한 후 해당 YAML 명세를 동기화하는 워크플로우입니다.

## Prerequisites
- GDC가 초기화된 프로젝트
- 수정된 코드가 `.gdc/nodes/`에 해당 YAML이 존재하는 노드

## Steps

1. **변경된 코드 확인**: 수정된 소스 파일을 확인합니다.
   - 어떤 노드(클래스/인터페이스)가 변경되었는지 파악
   - 새로 추가된 메서드, 수정된 시그니처, 의존성 변경 등 확인

// turbo
2. **현재 명세 상태 확인**: 변경 전 YAML 명세를 확인합니다.
   ```
   gdc show <NodeName>
   ```

// turbo
3. **코드 → YAML 동기화**: 코드에서 YAML을 자동 생성/업데이트합니다.
   ```
   gdc sync --direction code
   ```

4. **동기화 결과 검토**: 생성/업데이트된 YAML을 확인합니다.
   - 새로 추출된 메서드 시그니처 확인
   - 의존성 목록이 올바른지 검증
   - `responsibility.summary` 등 수동 필드는 직접 보완

// turbo
5. **정합성 검증**: 전체 그래프의 일관성을 확인합니다.
   ```
   gdc check
   ```

// turbo
6. **DB 동기화**: 변경사항을 SQLite DB에 반영합니다.
   ```
   gdc sync --direction yaml
   ```

## Example Usage

```
/update-spec
```

## Notes
- `gdc sync --direction code`는 소스 코드를 파싱하여 YAML 생성
- 자동 생성된 YAML의 `responsibility.summary`는 수동으로 보완 권장
- 새 노드 발견 시 자동으로 YAML 파일 생성
