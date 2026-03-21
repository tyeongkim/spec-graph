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

| Type | Prefix | 설명 | 예시 |
|---|---|---|---|
| `requirement` | REQ | 기능/비기능 요구사항 | REQ-001 |
| `decision` | DEC | 정책/아키텍처 결정 | DEC-003 |
| `phase` | PHS | 개발 phase 또는 milestone | PHS-002 |
| `interface` | API | API 계약, 모듈 인터페이스, 이벤트 계약 | API-005 |
| `state` | STT | 상태 또는 상태 전이 규칙 | STT-001 |
| `test` | TST | 테스트 케이스/시나리오 | TST-012 |
| `crosscut` | XCT | 권한, 감사, 멱등성 등 횡단 관심사 | XCT-002 |
| `question` | QST | 아직 닫히지 않은 질문 | QST-004 |
| `assumption` | ASM | 아직 검증되지 않은 가정 | ASM-003 |
| `criterion` | ACT | acceptance criterion | ACT-009 |
| `risk` | RSK | 명시적 리스크 항목 | RSK-002 |

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
        'requirement', 'decision', 'phase', 'interface', 'state', 'test',
        'crosscut', 'question', 'assumption', 'criterion', 'risk'
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

이 설계의 핵심은 relation semantics를 분리하는 것이다.
기존의 `belongs_to` 하나로 phase 귀속, 스코프, 구현 전달을 동시에 표현하지 않는다.

### Relation Types

| Relation | Meaning |
|---|---|
| `implements` | 구현체가 requirement 또는 criterion을 구현한다 |
| `verifies` | test가 requirement / criterion / decision을 검증한다 |
| `depends_on` | from이 to에 의존한다 |
| `constrained_by` | from이 constraint성 개체(crosscut / decision / assumption)에 의해 제약된다 |
| `planned_in` | requirement / decision / interface / test가 특정 phase에 포함된다 |
| `delivered_in` | 구현 산출물이 특정 phase에서 실제로 전달된다 |
| `triggers` | interface 또는 decision이 상태 전이를 유발한다 |
| `answers` | decision이 question을 해소한다 |
| `assumes` | requirement / decision / phase가 assumption에 의존한다 |
| `has_criterion` | requirement가 acceptance criterion을 가진다 |
| `mitigates` | decision / test / crosscut이 risk를 완화한다 |
| `supersedes` | 새 개체가 이전 개체를 대체한다 |
| `conflicts_with` | 두 개체가 의미적으로 충돌한다 |
| `references` | 약한 참조 관계 |

### Allowed edge matrix

자유로운 edge 생성을 막기 위해, relation마다 허용 source/target 타입을 고정한다.

| Relation | From | To |
|---|---|---|
| `implements` | `interface` | `requirement`, `criterion` |
| `verifies` | `test` | `requirement`, `criterion`, `decision`, `interface`, `state` |
| `depends_on` | `requirement`, `decision`, `interface`, `phase`, `test`, `state` | `requirement`, `decision`, `interface`, `state`, `crosscut`, `assumption` |
| `constrained_by` | `requirement`, `decision`, `interface`, `phase`, `state` | `crosscut`, `decision`, `assumption` |
| `planned_in` | `requirement`, `decision`, `interface`, `test`, `question`, `risk` | `phase` |
| `delivered_in` | `interface`, `state`, `test`, `decision` | `phase` |
| `triggers` | `interface`, `decision` | `state` |
| `answers` | `decision` | `question` |
| `assumes` | `requirement`, `decision`, `phase`, `interface` | `assumption` |
| `has_criterion` | `requirement` | `criterion` |
| `mitigates` | `decision`, `test`, `crosscut`, `phase` | `risk` |
| `supersedes` | same type only | same type only |
| `conflicts_with` | same or related semantic types | same or related semantic types |
| `references` | any | any |

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
        'planned_in', 'delivered_in', 'triggers', 'answers', 'assumes',
        'has_criterion', 'mitigates', 'supersedes', 'conflicts_with', 'references'
    ))
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
spec-graph relation add --from REQ-001 --to PHS-002 --type planned_in
spec-graph relation add --from API-005 --to PHS-002 --type delivered_in
spec-graph relation list --from API-005
spec-graph relation delete --from API-005 --to REQ-001 --type implements
```

## Impact

```bash
spec-graph impact REQ-001
spec-graph impact REQ-001 DEC-003
spec-graph impact REQ-001 --follow implements,verifies,planned_in
spec-graph impact REQ-001 --min-severity medium
spec-graph impact REQ-001 --dimension structural
```

## Validate

```bash
spec-graph validate
spec-graph validate --check orphans
spec-graph validate --check coverage
spec-graph validate --check cycles
spec-graph validate --check conflicts
spec-graph validate --check gates
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

| Relation | Direction | Structural | Behavioral | Planning | Note |
|---|---|---:|---:|---:|---|
| `implements` | bidirectional | 0.9 | 0.8 | 0.4 | requirement ↔ implementation 강결합 |
| `verifies` | from test -> target, reverse weak | 0.4 | 0.8 | 0.3 | 테스트는 동작 영향 강함 |
| `depends_on` | from -> to | 0.8 | 0.7 | 0.4 | 의존성 방향만 강함 |
| `constrained_by` | from -> to | 0.5 | 0.8 | 0.4 | 정책/제약 영향 |
| `planned_in` | from -> to | 0.1 | 0.2 | 0.8 | planning 의미 위주 |
| `delivered_in` | from -> to | 0.3 | 0.3 | 0.9 | 실제 delivery 영향 |
| `triggers` | from -> to | 0.6 | 0.9 | 0.2 | 상태 전이 영향 강함 |
| `answers` | from -> to | 0.2 | 0.7 | 0.3 | decision이 question 해결 |
| `assumes` | from -> to | 0.3 | 0.8 | 0.5 | 가정 변경은 의미 영향 큼 |
| `has_criterion` | bidirectional | 0.3 | 0.9 | 0.2 | criterion과 requirement 강결합 |
| `mitigates` | from -> to | 0.2 | 0.6 | 0.4 | risk 관점 |
| `supersedes` | new -> old, reverse weak | 0.4 | 0.5 | 0.3 | 교체 추적 |
| `conflicts_with` | bidirectional | 0.8 | 0.9 | 0.5 | 충돌은 매우 강함 |
| `references` | bidirectional weak | 0.1 | 0.1 | 0.1 | 약한 참조 |

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

### Core graph checks

- `orphans`: 아무 관계도 없는 entity
- `cycles`: 허용되지 않은 순환 참조
- `conflicts`: `conflicts_with` 또는 정책 충돌
- `invalid_edges`: allowed edge matrix 위반
- `superseded_refs`: deprecated / superseded entity를 active item이 계속 참조

### Coverage checks

- active requirement에 `implements`가 없음
- active requirement에 `has_criterion`이 없음
- criterion에 `verifies`가 없음
- state를 유발하는 interface에 관련 test가 없음

### Workflow gate checks

- phase 안에 unresolved question이 남아 있음
- high-risk item이 mitigates 없이 phase에 포함됨
- draft decision에 의존하는 active requirement가 존재함
- assumption에 의존하지만 검증 계획이 없음
- phase 완료 시 delivered item은 있는데 planned item의 핵심 requirement가 누락됨

### Suggested checks for later

- permission / audit / idempotency 같은 crosscut 적용 여부
- state transition coverage completeness
- migration required but no phase or interface linkage

---

## Agent Integration Patterns

### Phase planning

```bash
spec-graph entity add --type phase --id PHS-003 --title "Phase 3 - Payment"
spec-graph relation add --from REQ-010 --to PHS-003 --type planned_in
spec-graph relation add --from REQ-011 --to PHS-003 --type planned_in
spec-graph validate --phase PHS-003 --check gates
```

### Change handling

```bash
spec-graph impact DEC-031 --dimension behavioral | jq '.affected[] | {id, impact, reason}'
spec-graph query unresolved --type question
spec-graph validate
```

### Phase exit

```bash
spec-graph query scope PHS-002
spec-graph validate --phase PHS-002 --check coverage
spec-graph validate --phase PHS-002 --check gates
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
│   │   ├── validate.go
│   │   ├── query.go
│   │   ├── export.go
│   │   └── rules.go
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
- [ ] SQLite schema
- [ ] Entity CRUD
- [ ] Relation CRUD
- [ ] Allowed edge matrix validation
- [ ] JSON stdout contracts

### v0.2 — Impact + validate core
- [ ] `impact` with dimensions
- [ ] `validate --check orphans`
- [ ] `validate --check coverage`
- [ ] `validate --check invalid_edges`
- [ ] `validate --check superseded_refs`

### v0.3 — Workflow gates
- [ ] `validate --check gates`
- [ ] phase-scoped validation
- [ ] unresolved questions / assumptions detection

### v0.4 — History + bootstrap
- [ ] changeset recording
- [ ] relation history
- [ ] `bootstrap scan`
- [ ] review-mode import

### v0.5 — Query / export / agent integration
- [ ] `query scope / path / unresolved`
- [ ] DOT / Mermaid export
- [ ] MCP server mode

---

## Final Position

이 설계의 핵심은 “문서를 더 잘 관리하는 도구”가 아니라, **의미 객체와 관계에 대해 계산 가능한 operator layer**를 만드는 것이다.

따라서 이 프로젝트의 성공 기준은 다음과 같다.

- Markdown 개수가 줄어드는가가 아니다.
- 그래프가 예뻐 보이는가도 아니다.
- **변경 시 영향을 빠짐없이 계산하고, phase 진행 전후의 누락을 기계적으로 잡아내는가**가 핵심이다.

한 문장으로 요약하면:

> `spec-graph`는 LLM이 문서를 “추론해서 맞추는” 방식을 줄이고, 시스템이 의미 관계를 “계산해서 알려주는” 방식으로 전환하기 위한 semantic operator layer다.
