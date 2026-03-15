-- ═══════════════════════════════════════════════════════════════════════════════
-- GDC Database Schema
-- SQLite3 (.gdc/graph.db)
-- Version: 1.0.0
-- ═══════════════════════════════════════════════════════════════════════════════

-- 성능 최적화 설정
PRAGMA journal_mode = WAL;          -- Write-Ahead Logging (동시성 향상)
PRAGMA foreign_keys = ON;           -- 외래 키 제약 활성화
PRAGMA synchronous = NORMAL;        -- 동기화 모드 (성능 vs 안전성 균형)


-- ═══════════════════════════════════════════════════════════════════════════════
-- 노드 테이블 (Nodes)
-- 그래프의 정점(Vertex)을 저장
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS nodes (
    -- 기본 정보
    id              TEXT PRIMARY KEY,                       -- 노드 고유 식별자
    type            TEXT NOT NULL CHECK(type IN (
                        'class', 'interface', 'module', 
                        'service', 'enum'
                    )),                                     -- 노드 타입
    layer           TEXT CHECK(layer IN (
                        'domain', 'application', 
                        'infrastructure', 'presentation'
                    )),                                     -- 아키텍처 레이어
    namespace       TEXT,                                   -- 네임스페이스/패키지
    
    -- 파일 경로
    spec_path       TEXT NOT NULL,                          -- YAML 명세 파일 경로
    impl_path       TEXT,                                   -- 구현 파일 경로
    
    -- 책임
    responsibility  TEXT,                                   -- 책임 요약
    
    -- 상태
    status          TEXT DEFAULT 'draft' CHECK(status IN (
                        'draft', 'specified', 'implemented', 
                        'tested', 'deprecated'
                    )),                                     -- 현재 상태
    
    -- 해시 (정합성 검사용)
    spec_hash       TEXT,                                   -- 명세(인터페이스) 해시
    impl_hash       TEXT,                                   -- 구현 코드 해시
    
    -- 타임스탬프
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 인덱스
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
CREATE INDEX IF NOT EXISTS idx_nodes_layer ON nodes(layer);
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_namespace ON nodes(namespace);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 엣지 테이블 (Edges / Dependencies)
-- 그래프의 간선(Edge)을 저장. 방향성 있음 (from → to)
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS edges (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- 연결 정보
    from_node       TEXT NOT NULL,                          -- 의존하는 노드
    to_node         TEXT NOT NULL,                          -- 의존 대상 노드
    
    -- 의존성 속성
    dependency_type TEXT NOT NULL CHECK(dependency_type IN (
                        'interface', 'class', 'service'
                    )),                                     -- 대상 타입
    injection_type  TEXT CHECK(injection_type IN (
                        'constructor', 'property', 'method'
                    )),                                     -- 주입 방식
    is_optional     BOOLEAN DEFAULT FALSE,                  -- 선택적 의존성 여부
    
    -- 계약 정보
    contract_hash   TEXT,                                   -- 의존 시점의 to_node.spec_hash
    usage_summary   TEXT,                                   -- 사용 방식 요약
    
    -- 타임스탬프
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    -- 제약조건
    FOREIGN KEY (from_node) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (to_node) REFERENCES nodes(id) ON DELETE CASCADE,
    UNIQUE(from_node, to_node)                              -- 동일 방향 중복 방지
);

-- 인덱스 (의존성 조회 최적화)
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_node);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_node);
CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(dependency_type);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 인터페이스 멤버 테이블 (Interface Members)
-- 각 노드가 제공하는 공개 인터페이스 멤버들
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS interface_members (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id         TEXT NOT NULL,                          -- 소속 노드
    
    -- 멤버 정보
    member_type     TEXT NOT NULL CHECK(member_type IN (
                        'constructor', 'method', 
                        'property', 'event'
                    )),                                     -- 멤버 타입
    name            TEXT NOT NULL,                          -- 멤버 이름
    signature       TEXT NOT NULL,                          -- 전체 시그니처
    description     TEXT,                                   -- 설명
    
    -- 타입 정보
    return_type     TEXT,                                   -- 반환 타입
    parameters_json TEXT,                                   -- 파라미터 (JSON 배열)
    
    -- 제약조건
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- 인덱스
CREATE INDEX IF NOT EXISTS idx_members_node ON interface_members(node_id);
CREATE INDEX IF NOT EXISTS idx_members_name ON interface_members(name);
CREATE INDEX IF NOT EXISTS idx_members_type ON interface_members(member_type);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 태그 테이블 (Tags)
-- 노드에 연결된 태그들 (다대다 관계)
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS tags (
    node_id         TEXT NOT NULL,
    tag             TEXT NOT NULL,
    
    PRIMARY KEY (node_id, tag),
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- 인덱스
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 동기화 로그 테이블 (Sync Log)
-- 동기화 및 변경 이력 추적
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS sync_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- 작업 정보
    action          TEXT NOT NULL CHECK(action IN (
                        'create', 'update', 'delete', 
                        'validate', 'sync'
                    )),                                     -- 작업 유형
    target_type     TEXT CHECK(target_type IN (
                        'node', 'edge', 'member'
                    )),                                     -- 대상 유형
    target_id       TEXT,                                   -- 대상 식별자
    target_file     TEXT,                                   -- 관련 파일 경로
    
    -- 변경 내용
    old_hash        TEXT,                                   -- 이전 해시
    new_hash        TEXT,                                   -- 새 해시
    message         TEXT,                                   -- 메시지
    
    -- 타임스탬프
    timestamp       DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 인덱스
CREATE INDEX IF NOT EXISTS idx_synclog_action ON sync_log(action);
CREATE INDEX IF NOT EXISTS idx_synclog_target ON sync_log(target_id);
CREATE INDEX IF NOT EXISTS idx_synclog_time ON sync_log(timestamp);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 검증 이슈 테이블 (Validation Issues)
-- 정합성 검사에서 발견된 이슈들
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS validation_issues (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- 분류
    severity        TEXT NOT NULL CHECK(severity IN (
                        'error', 'warning', 'info'
                    )),                                     -- 심각도
    category        TEXT NOT NULL CHECK(category IN (
                        'missing_ref',      -- 참조 대상 없음
                        'hash_mismatch',    -- 해시 불일치
                        'cycle',            -- 순환 참조
                        'orphan',           -- 고립된 노드
                        'srp_violation',    -- SRP 위반
                        'layer_violation',  -- 레이어 아키텍처 위반
                        'missing_impl',     -- 구현 없음
                        'schema_error'      -- 스키마 유효성 오류
                    )),                                     -- 이슈 카테고리
    
    -- 관련 노드
    source_node     TEXT,                                   -- 발생 노드
    target_node     TEXT,                                   -- 관련 대상 노드
    
    -- 상세 정보
    message         TEXT NOT NULL,                          -- 이슈 메시지
    suggestion      TEXT,                                   -- 해결 제안
    details_json    TEXT,                                   -- 추가 상세 정보 (JSON)
    
    -- 상태
    resolved        BOOLEAN DEFAULT FALSE,                  -- 해결 여부
    resolved_at     DATETIME,                               -- 해결 시간
    
    -- 타임스탬프
    detected_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    -- 외래 키 (선택적, 노드가 삭제되어도 이슈는 유지)
    FOREIGN KEY (source_node) REFERENCES nodes(id) ON DELETE SET NULL,
    FOREIGN KEY (target_node) REFERENCES nodes(id) ON DELETE SET NULL
);

-- 인덱스
CREATE INDEX IF NOT EXISTS idx_issues_severity ON validation_issues(severity);
CREATE INDEX IF NOT EXISTS idx_issues_category ON validation_issues(category);
CREATE INDEX IF NOT EXISTS idx_issues_resolved ON validation_issues(resolved);
CREATE INDEX IF NOT EXISTS idx_issues_source ON validation_issues(source_node);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 메타데이터 테이블 (Metadata)
-- 데이터베이스/프로젝트 수준의 메타정보
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS metadata (
    key             TEXT PRIMARY KEY,
    value           TEXT,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 기본 메타데이터 삽입
INSERT OR REPLACE INTO metadata (key, value) VALUES 
    ('schema_version', '1.0.0'),
    ('created_at', datetime('now')),
    ('project_name', NULL),
    ('project_language', NULL);


-- ═══════════════════════════════════════════════════════════════════════════════
-- 뷰 (Views) - 자주 사용하는 쿼리를 뷰로 정의
-- ═══════════════════════════════════════════════════════════════════════════════

-- 고립된 노드 뷰 (orphan nodes)
CREATE VIEW IF NOT EXISTS v_orphan_nodes AS
SELECT n.id, n.type, n.layer, n.status
FROM nodes n
LEFT JOIN edges e ON n.id = e.to_node
WHERE e.id IS NULL
  AND n.type NOT IN ('interface', 'enum');

-- 해시 불일치 뷰 (contract violations)
CREATE VIEW IF NOT EXISTS v_contract_violations AS
SELECT 
    e.from_node,
    e.to_node,
    e.contract_hash AS expected_hash,
    n.spec_hash AS current_hash
FROM edges e
JOIN nodes n ON e.to_node = n.id
WHERE e.contract_hash IS NOT NULL 
  AND e.contract_hash != n.spec_hash;

-- 레이어별 노드 통계 뷰
CREATE VIEW IF NOT EXISTS v_layer_stats AS
SELECT 
    COALESCE(layer, 'unknown') as layer,
    COUNT(*) as total_nodes,
    SUM(CASE WHEN status = 'implemented' THEN 1 ELSE 0 END) as implemented,
    SUM(CASE WHEN status = 'tested' THEN 1 ELSE 0 END) as tested
FROM nodes
GROUP BY layer;

-- 의존성 요약 뷰
CREATE VIEW IF NOT EXISTS v_dependency_summary AS
SELECT 
    n.id,
    n.type,
    n.layer,
    COUNT(DISTINCT e_out.id) as outgoing_deps,
    COUNT(DISTINCT e_in.id) as incoming_refs
FROM nodes n
LEFT JOIN edges e_out ON n.id = e_out.from_node
LEFT JOIN edges e_in ON n.id = e_in.to_node
GROUP BY n.id;


-- ═══════════════════════════════════════════════════════════════════════════════
-- 인덱스 추가 (최적화)
-- ═══════════════════════════════════════════════════════════════════════════════

-- Full-text search를 위한 FTS 테이블 (선택적)
-- CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
--     id, 
--     responsibility, 
--     content='nodes', 
--     content_rowid='rowid'
-- );
