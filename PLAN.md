# spec-graph: Semantic Operator for LLM Agent Workflows

## Overview

`spec-graph`는 phase 기반 개발 프로세스 위에 **typed semantic graph**를 추가하는 CLI 도구다.
이 도구의 목적은 문서를 더 잘 쓰는 것이 아니라, **요구사항·결정·phase·인터페이스·상태·테스트 사이의 의미 관계를 구조화하고, 변경 영향과 누락을 계산 가능하게 만드는 것**이다.

핵심 목표는 네 가지다.

1. **Impact Analysis** — 어떤 개체가 바뀌면 무엇이 함께 바뀌어야 하는지 계산한다.
2. **Gap Detection** — 구현/검증/계획/질문 누락을 탐지한다.
3. **Consistency Validation** — 그래프 정합성과 workflow gate를 검사한다.
4. **Agent Coordination** — LLM은 자유 텍스트 전체를 추론하는 대신, 영향 집합과 패치 대상 목록을 받아 작업한다.

이 설계의 핵심 원칙은 다음과 같다.

- 마크다운은 설명 뷰이고, **구조화된 그래프가 source of truth**다.
- 그래프는 자유 그래프가 아니라, **허용된 타입과 관계만 갖는 constrained typed graph**다.
- `impact`는 단순 BFS가 아니라, **relation semantics + impact dimension**을 반영한다.
- `validate`는 문서 lint가 아니라, **phase 진행 전후의 gate** 역할을 한다.
- `bootstrap`은 자동 import가 아니라, **candidate extraction**이다.

---

## Architecture

```text
┌──────────────────────────────────────────────────────────┐
│  LLM Agent / Automation                                  │
│  - reads JSON output                                     │
│  - asks spec-graph for impact / gaps / scope             │
│  - patches only returned targets                         │
└───────────────────────┬──────────────────────────────────┘
                        │ CLI / JSON
                        ▼
┌──────────────────────────────────────────────────────────┐
│  spec-graph CLI                                           │
│                                                           │
│  init        ─ 프로젝트 초기화                           │
│  entity      ─ 개체 CRUD                                  │
│  relation    ─ 관계 CRUD                                  │
│  impact      ─ 변경 영향 분석                             │
│  validate    ─ 그래프/워크플로우 검증                    │
│  query       ─ 탐색 및 조회                               │
│  export      ─ 전체/부분 그래프 export                    │
│  history     ─ 변경 이력 및 change-set 조회               │
│  bootstrap   ─ 문서에서 후보 개체/관계 추출              │
└───────────────────────┬──────────────────────────────────┘
                        │ read / write
                        ▼
┌──────────────────────────────────────────────────────────┐
│  SQLite (.spec-graph/graph.db)                           │
│                                                           │
│  entities        ─ typed nodes                            │
│  relations       ─ typed edges                            │
│  changesets      ─ 변경 단위                              │
│  entity_history  ─ 개체 이력                              │
│  relation_history─ 관계 이력                              │
│  validations     ─ 검증 결과 캐시(선택)                  │
└──────────────────────────────────────────────────────────┘
```

---

## Data Model

## Entity Types

초기 MVP부터 아래 타입을 지원한다.

v1부터 entity는 세 레이어로 분류된다: **arch** (의미 레이어), **exec** (실행 레이어), **mapping** (교차 레이어).

**arch 레이어** — 시스템의 "무엇"과 "왜"

| Type | Prefix | 설명 | 예시 |
|---|---|---|---|
| `requirement` | REQ | 기능/비기능 요구사항 | REQ-001 |
| `decision` | DEC | 정책/아키텍처 결정 | DEC-003 |
| `interface` | API | API 계약, 모듈 인터페이스, 이벤트 계약 | API-005 |
| `state` | STT | 상태 또는 상태 전이 규칙 | STT-001 |
| `test` | TST | 테스트 케이스/시나리오 | TST-012 |
| `crosscut` | XCT | 권한, 감사, 멱등성 등 횡단 관심사 | XCT-002 |
| `question` | QST | 아직 닫히지 않은 질문 | QST-004 |
| `assumption` | ASM | 아직 검증되지 않은 가정 | ASM-003 |
| `criterion` | ACT | acceptance criterion | ACT-009 |
| `risk` | RSK | 명시적 리스크 항목 | RSK-002 |

**exec 레이어** — 시스템의 "언제"와 "어떻게"

| Type | Prefix | 설명 | 예시 |
|---|---|---|---|
| `plan` | PLN | phase를 묶는 단일 활성 delivery 계획 | PLN-001 |
| `phase` | PHS | 개발 phase 또는 milestone | PHS-002 |

### Entity Status

모든 entity는 아래 상태 중 하나를 가진다.

- `draft`
- `active`
- `deprecated`
- `resolved` (question / risk / assumption용)
- `deleted`

### Entity Schema (SQLite)

```sql
CREATE TABLE entities (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    title        TEXT NOT NULL,
    description  TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    metadata     TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (type IN (
        'requirement', 'decision', 'interface', 'state', 'test',
        'crosscut', 'question', 'assumption', 'criterion', 'risk',
        'plan', 'phase'
    )),
    CHECK (status IN ('draft', 'active', 'deprecated', 'resolved', 'deleted'))
);
```

### Type-specific metadata

`metadata`는 JSON이지만, 타입별 최소 필수 필드를 가진다.

예시:

- `requirement`
  - `priority`: `must | should | could`
  - `kind`: `functional | non_functional`
  - `owner`: string
- `decision`
  - `rationale`: string
  - `date`: ISO8601
- `phase`
  - `goal`: string
  - `order`: integer
  - `exit_criteria`: string[]
- `plan`
  - `status`: `active | archived`
- `interface`
  - `kind`: `http | event | module | storage`
- `state`
  - `entity`: string
  - `from`: string
  - `to`: string
- `test`
  - `kind`: `unit | integration | e2e | property`
- `question`
  - `owner`: string
  - `due_at`: ISO8601?
- `assumption`
  - `confidence`: `low | medium | high`
- `criterion`
  - `given`: string
  - `when`: string
  - `then`: string

---

## Relation Model

이 설계의 핵심은 relation semantics를 레이어별로 분리하는 것이다.
v1부터 relation은 세 레이어로 나뉜다: **arch** (12개), **exec** (3개), **mapping** (2개).

### Relation Types

**arch 레이어 (12개)**

| Relation | Meaning |
|---|---|
| `implements` | 구현체가 requirement 또는 criterion을 구현한다 |
| `verifies` | test가 requirement / criterion / decision을 검증한다 |
| `depends_on` | from이 to에 의존한다 |
| `constrained_by` | from이 constraint성 개체(crosscut / decision / assumption)에 의해 제약된다 |
| `triggers` | interface 또는 decision이 상태 전이를 유발한다 |
| `answers` | decision이 question을 해소한다 |
| `assumes` | requirement / decision / interface가 assumption에 의존한다 |
| `has_criterion` | requirement가 acceptance criterion을 가진다 |
| `mitigates` | decision / test / crosscut이 risk를 완화한다 |
| `supersedes` | 새 개체가 이전 개체를 대체한다 |
| `conflicts_with` | 두 개체가 의미적으로 충돌한다 |
| `references` | 약한 참조 관계 |

**exec 레이어 (3개)**

| Relation | Meaning |
|---|---|
| `belongs_to` | phase가 plan에 귀속된다 |
| `precedes` | phase가 다른 phase보다 먼저 실행된다 |
| `blocks` | phase가 다른 phase의 시작을 막는다 |

**mapping 레이어 (2개)**

| Relation | Meaning |
|---|---|
| `covers` | phase가 arch entity를 커버한다 (계획 의도) |
| `delivers` | phase가 arch entity를 실제로 전달한다 (완료 증거) |

> `planned_in` / `delivered_in`은 v1에서 `covers` / `delivers`로 대체됨. 방향이 반전됨: arch→phase 였던 것이 phase→arch로 변경.

### Allowed edge matrix

자유로운 edge 생성을 막기 위해, relation마다 허용 source/target 타입을 고정한다.

**arch 레이어**

| Relation | From | To |
|---|---|---|
| `implements` | `interface` | `requirement`, `criterion` |
| `verifies` | `test` | `requirement`, `criterion`, `decision`, `interface`, `state` |
| `depends_on` | `requirement`, `decision`, `interface`, `test`, `state` | `requirement`, `decision`, `interface`, `state`, `crosscut`, `assumption` |
| `constrained_by` | `requirement`, `decision`, `interface`, `state` | `crosscut`, `decision`, `assumption` |
| `triggers` | `interface`, `decision` | `state` |
| `answers` | `decision` | `question` |
| `assumes` | `requirement`, `decision`, `interface` | `assumption` |
| `has_criterion` | `requirement` | `criterion` |
| `mitigates` | `decision`, `test`, `crosscut` | `risk` |
| `supersedes` | same arch type only | same arch type only |
| `conflicts_with` | same or related arch types | same or related arch types |
| `references` | any arch type | any arch type |

**exec 레이어**

| Relation | From | To |
|---|---|---|
| `belongs_to` | `phase` | `plan` |
| `precedes` | `phase` | `phase` |
| `blocks` | `phase` | `phase` |

**mapping 레이어**

| Relation | From | To |
|---|---|---|
| `covers` | `phase` | arch entity (requirement, decision, interface, state, test, crosscut, question, assumption, criterion, risk) |
| `delivers` | `phase` | arch entity (requirement, decision, interface, state, test, crosscut, question, assumption, criterion, risk) |

### Relation Schema (SQLite)

```sql
CREATE TABLE relations (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id      TEXT NOT NULL REFERENCES entities(id),
    to_id        TEXT NOT NULL REFERENCES entities(id),
    type         TEXT NOT NULL,
    weight       REAL NOT NULL DEFAULT 1.0,
    metadata     TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(from_id, to_id, type),
    CHECK (type IN (
        'implements', 'verifies', 'depends_on', 'constrained_by',
        'triggers', 'answers', 'assumes',
        'has_criterion', 'mitigates', 'supersedes', 'conflicts_with', 'references',
        'belongs_to', 'precedes', 'blocks',
        'covers', 'delivers'
    ))
    -- v1: planned_in/delivered_in은 covers/delivers로 대체됨 (방향 반전: arch→phase → phase→arch)
);
```

### Relation validation rule

`relation add` 시 아래를 반드시 검사한다.

1. source/target entity 존재 여부
2. relation type 허용 여부
3. allowed edge matrix 충족 여부
4. 중복 edge 여부
5. self-loop 허용 여부 (`conflicts_with`, `supersedes` 등은 금지)

---

## Change Tracking

단순 audit log만으로는 부족하므로, **change set** 단위를 추가한다.

### Changeset Schema

```sql
CREATE TABLE changesets (
    id           TEXT PRIMARY KEY,
    reason       TEXT NOT NULL,
    actor        TEXT,
    source       TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### Entity History

```sql
CREATE TABLE entity_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    changeset_id  TEXT NOT NULL REFERENCES changesets(id),
    entity_id     TEXT NOT NULL,
    action        TEXT NOT NULL,
    before_json   TEXT,
    after_json    TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### Relation History

```sql
CREATE TABLE relation_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    changeset_id  TEXT NOT NULL REFERENCES changesets(id),
    relation_key  TEXT NOT NULL,
    action        TEXT NOT NULL,
    before_json   TEXT,
    after_json    TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
```

이 구조를 쓰면 다음이 가능하다.

- 특정 결정 변경이 어떤 entity / relation을 함께 바꿨는지 추적
- impact 계산 결과와 실제 반영 범위 비교
- phase 도중 긴급 변경의 파급 복원

---

## CLI Interface

모든 출력은 JSON으로 stdout에 출력한다. 에러는 stderr. 종료 코드는 다음을 따른다.

- `0`: 성공
- `1`: 실행 에러
- `2`: 검증 실패
- `3`: invalid input / schema violation

## Init

```bash
spec-graph init
spec-graph init --path /custom/path
```

## Entity

```bash
spec-graph entity add --type requirement --id REQ-001 \
  --title "모든 API는 인증 필요" \
  --description "익명 접근 금지" \
  --metadata '{"priority":"must","kind":"functional"}'

spec-graph entity get REQ-001
spec-graph entity list --type requirement --status active
spec-graph entity update REQ-001 --title "새 제목" --reason "기획 변경"
spec-graph entity deprecate REQ-001 --reason "REQ-015로 대체"
spec-graph entity delete REQ-001
```

## Relation

```bash
spec-graph relation add --from API-005 --to REQ-001 --type implements
spec-graph relation add --from TST-012 --to REQ-001 --type verifies
spec-graph relation add --from PHS-002 --to REQ-001 --type covers    # v1: phase→arch (구 planned_in은 arch→phase였음)
spec-graph relation add --from PHS-002 --to API-005 --type delivers  # v1: phase→arch (구 delivered_in은 arch→phase였음)
spec-graph relation add --from PHS-002 --to PLN-001 --type belongs_to
spec-graph relation add --from PHS-001 --to PHS-002 --type precedes
spec-graph relation list --from API-005
spec-graph relation delete --from API-005 --to REQ-001 --type implements
```

## Impact

```bash
spec-graph impact REQ-001
spec-graph impact REQ-001 DEC-003
spec-graph impact REQ-001 --follow implements,verifies,covers
spec-graph impact REQ-001 --min-severity medium
spec-graph impact REQ-001 --dimension structural
```

## Validate

```bash
spec-graph validate
spec-graph validate --layer arch
spec-graph validate --layer exec
spec-graph validate --layer mapping
spec-graph validate --layer arch --check orphans
spec-graph validate --layer arch --check coverage
spec-graph validate --layer arch --check cycles
spec-graph validate --layer arch --check conflicts
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
spec-graph validate --layer mapping --phase PHS-002 --check delivery_completeness
spec-graph validate --layer mapping --phase PHS-002 --check mapping_consistency
spec-graph validate --phase PHS-002
spec-graph validate --entity REQ-001
```

## Query

```bash
spec-graph query neighbors REQ-001 --depth 2
spec-graph query path REQ-001 TST-012
spec-graph query scope PHS-002
spec-graph query unresolved --type question
spec-graph query sql "SELECT id, type FROM entities WHERE status = 'draft'"
```

## History

```bash
spec-graph history changeset CHG-001
spec-graph history entity REQ-001
spec-graph history relation API-005:REQ-001:implements
```

## Export

```bash
spec-graph export --format json
spec-graph export --format dot
spec-graph export --format mermaid
spec-graph export --center REQ-001 --depth 3 --format json
```

## Bootstrap

```bash
spec-graph bootstrap scan --input ./docs/
spec-graph bootstrap scan --input ./docs/ --format json
spec-graph bootstrap import --input extracted.json --mode review
```

`bootstrap import`의 기본 모드는 자동 반영이 아니라 **review**다.
즉, 추출된 후보를 보여주고 사용자가 승인하거나 에이전트가 후처리한 뒤 반영한다.

---

## JSON Output Contracts

에이전트와의 안정적인 연동을 위해, 주요 커맨드는 출력 shape를 고정한다.

### `impact` response

```json
{
  "sources": ["REQ-001"],
  "affected": [
    {
      "id": "API-005",
      "type": "interface",
      "depth": 1,
      "path": ["REQ-001", "API-005"],
      "relation_chain": ["implements"],
      "impact": {
        "overall": "high",
        "structural": "high",
        "behavioral": "medium",
        "planning": "low"
      },
      "reason": "직접 구현체"
    }
  ],
  "summary": {
    "total": 1,
    "by_type": {"interface": 1},
    "by_impact": {"high": 1}
  }
}
```

### `validate` response

```json
{
  "valid": false,
  "issues": [
    {
      "check": "coverage",
      "severity": "high",
      "entity": "REQ-007",
      "message": "구현체 없음"
    },
    {
      "check": "gates",
      "severity": "high",
      "entity": "PHS-002",
      "message": "해결되지 않은 question이 남아 있는데 phase를 완료하려고 함"
    }
  ],
  "summary": {
    "total_issues": 2,
    "by_severity": {"high": 2}
  }
}
```

---

## Impact Analysis Model

기존의 단일 severity scalar 대신, 최소 3개 축으로 나눈다.

- `structural`: 인터페이스, 상태, 의존성 구조에 미치는 영향
- `behavioral`: 정책, acceptance, 동작 의미에 미치는 영향
- `planning`: phase, 일정, delivery scope에 미치는 영향

### Relation propagation semantics

**arch 레이어**

| Relation | Direction | Structural | Behavioral | Planning | Note |
|---|---|---:|---:|---:|---|
| `implements` | bidirectional | 0.9 | 0.8 | 0.4 | requirement ↔ implementation 강결합 |
| `verifies` | from test -> target, reverse weak | 0.4 | 0.8 | 0.3 | 테스트는 동작 영향 강함 |
| `depends_on` | from -> to | 0.8 | 0.7 | 0.4 | 의존성 방향만 강함 |
| `constrained_by` | from -> to | 0.5 | 0.8 | 0.4 | 정책/제약 영향 |
| `triggers` | from -> to | 0.6 | 0.9 | 0.2 | 상태 전이 영향 강함 |
| `answers` | from -> to | 0.2 | 0.7 | 0.3 | decision이 question 해결 |
| `assumes` | from -> to | 0.3 | 0.8 | 0.5 | 가정 변경은 의미 영향 큼 |
| `has_criterion` | bidirectional | 0.3 | 0.9 | 0.2 | criterion과 requirement 강결합 |
| `mitigates` | from -> to | 0.2 | 0.6 | 0.4 | risk 관점 |
| `supersedes` | new -> old, reverse weak | 0.4 | 0.5 | 0.3 | 교체 추적 |
| `conflicts_with` | bidirectional | 0.8 | 0.9 | 0.5 | 충돌은 매우 강함 |
| `references` | bidirectional weak | 0.1 | 0.1 | 0.1 | 약한 참조 |

**exec 레이어**

| Relation | Direction | Structural | Behavioral | Planning | Note |
|---|---|---:|---:|---:|---|
| `belongs_to` | from -> to | 0.1 | 0.1 | 0.7 | phase → plan 귀속 |
| `precedes` | from -> to | 0.1 | 0.1 | 0.8 | 순서 의존성 |
| `blocks` | from -> to | 0.2 | 0.2 | 0.9 | 강한 순서 제약 |

**mapping 레이어**

| Relation | Direction | Structural | Behavioral | Planning | Note |
|---|---|---:|---:|---:|---|
| `covers` | phase -> arch | 0.1 | 0.2 | 0.8 | planning 의미 위주 (구 planned_in 대체) |
| `delivers` | phase -> arch | 0.3 | 0.3 | 0.9 | 실제 delivery 영향 (구 delivered_in 대체) |

### Algorithm sketch

```text
impact(sources, options):
  visited = best_score_per_node
  queue = initialize(sources)

  while queue not empty:
    current = pop_highest_score(queue)

    for rel in outgoing_and_allowed_reverse_edges(current):
      neighbor = rel.other_end(current)
      score = propagate(current.score, rel, options.dimension)

      if score < threshold:
        continue

      if score better than visited[neighbor]:
        visited[neighbor] = score
        enqueue(neighbor)

  return build_report(visited)
```

핵심은 다음과 같다.

- 무조건 양방향 전파하지 않는다.
- relation별 방향성과 impact dimension을 따로 둔다.
- 가장 강한 경로를 우선 사용한다.
- `references` 같은 약한 edge가 전체 결과를 오염시키지 않게 한다.

---

## Validation Checks

`validate`는 단순 그래프 lint가 아니라 workflow gate 역할을 수행한다.
v1부터 검증은 세 레이어로 분리된다.

### arch 레이어 검증 (`--layer arch`)

**Core graph checks**

- `orphans`: 아무 관계도 없는 arch entity
- `cycles`: 허용되지 않은 순환 참조 (depends_on 체인)
- `conflicts`: `conflicts_with` 또는 정책 충돌
- `invalid_edges`: allowed edge matrix 위반
- `superseded_refs`: deprecated / superseded entity를 active item이 계속 참조

**Coverage checks**

- active requirement에 `implements`가 없음
- active requirement에 `has_criterion`이 없음
- criterion에 `verifies`가 없음
- state를 유발하는 interface에 관련 test가 없음

**Unresolved checks**

- open question / unverified assumption / unmitigated risk 탐지

### exec 레이어 검증 (`--layer exec`)

- `phase_order`: precedes/blocks 체인이 유효한 순서를 형성하는지
- `single_active_plan`: 활성 plan이 정확히 하나인지
- `orphan_phases`: 어떤 plan에도 속하지 않는 phase
- `exec_cycles`: precedes/blocks 순환 참조
- `invalid_exec_edges`: exec edge matrix 위반

### mapping 레이어 검증 (`--layer mapping`)

- `plan_coverage`: 모든 active requirement가 어떤 phase에 의해 covers되는지
- `delivery_completeness`: covers된 arch entity에 delivers 증거가 있는지 (phase exit 필수)
- `mapping_consistency`: covers/delivers 대상이 실제 arch entity인지
- `invalid_mapping_edges`: mapping edge matrix 위반

### Workflow gate checks

- phase 안에 unresolved question이 남아 있음
- high-risk item이 mitigates 없이 phase에 포함됨
- draft decision에 의존하는 active requirement가 존재함
- assumption에 의존하지만 검증 계획이 없음
- phase 완료 시 delivers 증거는 있는데 covers된 핵심 requirement가 누락됨

### Suggested checks for later

- permission / audit / idempotency 같은 crosscut 적용 여부
- state transition coverage completeness
- migration required but no phase or interface linkage

---

## Agent Integration Patterns

### Phase planning

```bash
# plan 생성 (활성 plan은 하나만 허용)
spec-graph entity add --type plan --id PLN-001 --title "v1 Delivery Plan" \
  --metadata '{"status":"active"}'

# phase 생성 및 plan 귀속
spec-graph entity add --type phase --id PHS-003 --title "Phase 3 - Payment" \
  --metadata '{"goal":"결제 시스템 구축","order":3,"exit_criteria":["Payment API 완료","E2E 테스트 통과"]}'
spec-graph relation add --from PHS-003 --to PLN-001 --type belongs_to
spec-graph relation add --from PHS-002 --to PHS-003 --type precedes

# arch entity를 phase에 매핑 (covers: 계획 의도)
spec-graph relation add --from PHS-003 --to REQ-010 --type covers
spec-graph relation add --from PHS-003 --to REQ-011 --type covers

# gate 검사
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
```

### Change handling

```bash
spec-graph impact DEC-031 --dimension behavioral | jq '.affected[] | {id, impact, reason}'
spec-graph query unresolved --type question
spec-graph validate
```

### Phase exit

```bash
# phase 스코프 확인
spec-graph query scope PHS-002

# arch 커버리지 검사
spec-graph validate --layer arch --check coverage

# delivers 증거 확인 (phase exit 필수)
spec-graph validate --layer mapping --phase PHS-002 --check delivery_completeness
spec-graph validate --layer mapping --phase PHS-002 --check mapping_consistency

# exec gate 검사
spec-graph validate --layer exec --check phase_order
```

### Patch orchestration with agent

권장 흐름은 다음과 같다.

1. 변경 대상 확정
2. `impact`로 영향 집합 계산
3. `validate`로 현재 깨진 규칙 확인
4. 에이전트가 영향 대상만 수정
5. `validate` 재실행
6. 필요 시 `history`로 change set 기록 검토

즉 에이전트는 전 문서를 감으로 수정하지 않고, **계산된 대상만 패치**한다.

---

## Bootstrap Strategy

`bootstrap`은 편리하지만 가장 위험한 부분이다. 따라서 다음 원칙을 둔다.

- `bootstrap scan`은 후보만 추출한다.
- 추출 결과는 `confidence`와 `source_span`을 포함해야 한다.
- `bootstrap import`는 기본적으로 `--mode review`다.
- low-confidence relation은 자동 import하지 않는다.

### Candidate JSON example

```json
{
  "entities": [
    {
      "id": "REQ-001",
      "type": "requirement",
      "title": "모든 API는 인증 필요",
      "confidence": 0.94,
      "source": "docs/auth.md#L12-L18"
    }
  ],
  "relations": [
    {
      "from": "API-005",
      "to": "REQ-001",
      "type": "implements",
      "confidence": 0.71,
      "source": "docs/api.md#L30-L42"
    }
  ]
}
```

---

## Go Project Structure

```text
spec-graph/
├── cmd/
│   └── spec-graph/
│       └── main.go
├── internal/
│   ├── cli/
│   ├── db/
│   │   ├── sqlite.go
│   │   └── migrations/
│   ├── model/
│   │   ├── entity.go
│   │   ├── relation.go
│   │   ├── changeset.go
│   │   └── validation.go
│   ├── store/
│   │   ├── entity_store.go
│   │   ├── relation_store.go
│   │   ├── changeset_store.go
│   │   └── history_store.go
│   ├── graph/
│   │   ├── impact.go
│   │   ├── query.go
│   │   ├── export.go
│   │   └── rules.go
│   ├── validate/
│   │   ├── arch.go      ─ arch 레이어 검증 (orphans, coverage, cycles, conflicts, ...)
│   │   ├── exec.go      ─ exec 레이어 검증 (phase_order, single_active_plan, ...)
│   │   ├── mapping.go   ─ mapping 레이어 검증 (plan_coverage, delivery_completeness, ...)
│   │   ├── runner.go    ─ 검증 오케스트레이션 및 레이어 라우팅
│   │   └── types.go     ─ ValidationIssue, ValidationResult 타입 정의
│   ├── bootstrap/
│   │   ├── scan.go
│   │   └── import.go
│   └── jsoncontract/
│       └── schema.go
├── go.mod
├── go.sum
└── README.md
```

### Dependencies

| Package | Usage |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `modernc.org/sqlite` | SQLite driver without CGo |
| `encoding/json` | JSON IO |
| `github.com/xeipuuv/gojsonschema` or custom validation | JSON contract checks |

기본 선택은 `modernc.org/sqlite`를 권장한다. 이유는 크로스 컴파일과 배포 편의성 때문이다.

---

## Implementation Priorities

### v0.1 — Core graph
- [x] SQLite schema
- [x] Entity CRUD
- [x] Relation CRUD
- [x] Allowed edge matrix validation
- [x] JSON stdout contracts

### v0.2 — Impact + validate core
- [x] `impact` with dimensions
- [x] `validate --check orphans`
- [x] `validate --check coverage`
- [x] `validate --check invalid_edges`
- [x] `validate --check superseded_refs`

### v0.3 — Workflow gates
- [x] `validate --check gates`
- [x] phase-scoped validation
- [x] unresolved questions / assumptions detection

### v0.4 — History + bootstrap
- [x] changeset recording
- [x] relation history
- [x] `bootstrap scan`
- [x] review-mode import

### v0.5 — Query / export / agent integration
- [x] `query scope / path / unresolved`
- [x] DOT / Mermaid export
- [x] MCP server mode

### v1.0 — 3-layer 재설계 (완료)
- [x] arch / exec / mapping 레이어 분리
- [x] `plan` entity 타입 추가 (PLN prefix, exec 레이어)
- [x] `covers` / `delivers` relation 추가 (phase→arch, mapping 레이어)
- [x] `belongs_to` / `precedes` / `blocks` relation 추가 (exec 레이어)
- [x] `planned_in` / `delivered_in` deprecated (covers/delivers로 대체, 방향 반전)
- [x] validate 서브패키지 분리 (arch.go, exec.go, mapping.go, runner.go, types.go)
- [x] `--layer arch|exec|mapping` 플래그 지원
- [x] `delivery_completeness` / `mapping_consistency` / `plan_coverage` 검증 추가
- [x] `single_active_plan` / `phase_order` / `orphan_phases` exec 검증 추가

---

## Final Position

이 설계의 핵심은 “문서를 더 잘 관리하는 도구”가 아니라, **의미 객체와 관계에 대해 계산 가능한 operator layer**를 만드는 것이다.

따라서 이 프로젝트의 성공 기준은 다음과 같다.

- Markdown 개수가 줄어드는가가 아니다.
- 그래프가 예뻐 보이는가도 아니다.
- **변경 시 영향을 빠짐없이 계산하고, phase 진행 전후의 누락을 기계적으로 잡아내는가**가 핵심이다.

한 문장으로 요약하면:

> `spec-graph`는 LLM이 문서를 “추론해서 맞추는” 방식을 줄이고, 시스템이 의미 관계를 “계산해서 알려주는” 방식으로 전환하기 위한 semantic operator layer다.
