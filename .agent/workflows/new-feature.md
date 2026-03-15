---
description: 새로운 기능을 YAML 명세부터 시작하여 구현하기
---

# New Feature Workflow

새로운 기능을 기획 단계(YAML 명세)부터 시작하여 구현까지 완료하는 워크플로우입니다.

## Prerequisites
- GDC가 초기화된 프로젝트
- 기능 요구사항이 정의됨

## Steps

1. **기능 분석**: 요구사항을 분석하여 필요한 노드들을 식별합니다.
   - 어떤 인터페이스가 필요한가?
   - 어떤 클래스/서비스가 구현해야 하는가?
   - 기존 노드와의 의존 관계는?

2. **YAML 명세 작성**: 각 노드의 YAML 명세를 먼저 작성합니다.
   ```
   gdc node add <NodeName> --type <class|interface|service>
   ```
   그 후 `.gdc/nodes/<NodeName>.yaml` 파일을 상세히 작성

// turbo
3. **명세 검증**: 작성한 명세의 유효성을 확인합니다.
   ```
   gdc check
   ```

// turbo
4. **DB 동기화**: 새 명세를 DB에 반영합니다.
   ```
   gdc sync --direction yaml
   ```

5. **의존성 그래프 확인**: 전체 구조를 시각화합니다.
   ```
   gdc graph --output feature-graph.dot
   ```

6. **순차적 구현**: 의존성 순서대로 코드를 구현합니다.
   - 먼저 인터페이스 정의
   - 그 다음 구현체 작성
   - 마지막으로 서비스 레이어 구현

// turbo
7. **최종 동기화 및 검증**:
   ```
   gdc sync --direction yaml
   gdc check
   ```

## Example Usage

```
/new-feature "사용자 인증 기능"
```

## Template: Feature Planning

```yaml
# 기능: [기능명]
# 설명: [간단한 설명]

필요한 노드:
  인터페이스:
    - IAuthService: 인증 관련 비즈니스 로직 인터페이스
    - ITokenRepository: 토큰 저장소 인터페이스
  
  클래스:
    - AuthService: IAuthService 구현체
    - JWTTokenRepository: ITokenRepository 구현체
  
  의존 관계:
    - AuthService → ITokenRepository
    - AuthService → IUserRepository (기존)
```

## Notes
- "명세 우선(Spec First)" 접근법 권장
- 복잡한 기능은 여러 단계로 나누어 구현
- 각 단계마다 `gdc check`로 정합성 확인
