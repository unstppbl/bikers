# Agent instructions

- Suggest best practices, even if they may require refactoring.
- Do not preserve backward compatibility.
- Choose the simplest implementation that fully meets the current requirements.
- Prefer established, well-maintained libraries over custom implementations.
- Fix the cause, not the symptom.
- Search with `rg` instead of `grep`, and list files with `rg --files` instead
  of `find`. Add `--no-ignore` to search ignored vendored Go modules.

## Production deployment

- Commit and push changes before deployment. Build production images on Jarvis
  from the clean repository checkout using `make deploy-jarvis`.
- For deployment, verification or rollback, read
  [repository deployments](docs/operations.md#repository-deployments-on-jarvis).
  Keep private runtime settings and persistent data outside the checkout.

## Backend package layout

Organize `internal/` by responsibility. Before changing code, identify the
package that owns the behavior.

| Responsibility | Package |
|---|---|
| Application rules, decisions, checks, suggestions, limits, and transaction coordination | `managers/<feature>` |
| Shared domain data, serialized types, value validation, vocabularies, and pure functions over domain values | `models` |
| Repository interfaces and their callback types | `database` |
| Queries, locks, transaction mechanics, concurrency guards, and mapping stored records to domain types | `database/<implementation>` |
| Calls to an external service, including transport settings and defaults | `clients/<service>` |
| Loading and validating application settings | `config` |
| Request decoding, routing, response mapping, and middleware | `resources/<transport>` |
| Queue polling, background loops, and retry schedules | `workers` |
| Dependency construction, wiring, startup, and shutdown | `app` |

### Package ownership

- Add code to an existing package when it belongs to that responsibility.
  Create `managers/<feature>` for a new application responsibility or a rule
  set shared by several managers. Create `clients/<service>` for a new
  external service.
- Name feature and service packages with short lowercase nouns. Keep them
  one level below `managers` or `clients`.
- Every Go file under `internal` belongs to one of these layers.
- Keep helpers with their owner. Shared domain values and value checks
  belong in `models`. Shared application rules belong in their owning
  manager, even when those rules are pure functions.
- Keep the package map and architecture documentation aligned with changes.

### Dependency direction

Imports between internal packages follow these rules:

| Package | May import |
|---|---|
| `models`, `config` | No other internal packages |
| `database` | `models` |
| `database/<implementation>` | `database`, `models` |
| `managers/<feature>` | `models`, `database`, other managers without cycles |
| `clients/<service>` | `models` |
| `resources/<transport>` | Managers and `models` |
| `workers` | Managers, `models`, and queue clients needed for polling |
| `app` | All layers |

- Keep configuration loading in `config`. Pass settings through `app`
  when constructing dependencies.
- Managers declare the interfaces through which they call external
  services. `app` injects the client implementations.
- Workers call application behavior through interfaces declared by the
  worker or the manager they use.
- Enforce dependency rules with an automated architecture check. Resolve
  violations by moving code to its owner or introducing an interface
  wired by `app`.

### Rules and persistence

- Managers own application decisions. Repository implementations own
  persistence mechanics.
- Managers coordinate transactions through contracts in `database`.
- When a repository owns an atomic operation, pass the manager's decision
  into that operation through a callback. The repository owns transaction
  boundaries, locks, and lock ordering.
- Run network, model, embedding, and other external calls before acquiring
  database locks. Revalidate the relevant state and assumptions under the
  locks before applying a change.
- Keep callbacks executed under locks limited to local decisions and
  persistence work.

### Tests and commands

- Keep tests beside the code they test, in the same package or its
  `<name>_test` package.
- Reuse package-local fakes. Keep test helpers within the package that owns
  the tests.
- Put command flags, input, output, execution, and wiring in `cmd/<tool>`.
  Commands call managers for application behavior.
- Extend an existing command before adding another command with the same
  responsibility.

### Moving code and changing architecture

- When relocating code, update every caller and delete the old location.
  Do not leave aliases or forwarding functions.
- Preserve serialization tags and field order when moving existing domain
  types. Treat serialization changes as separate behavior changes.
- When moving rules out of persistence code, put decisions in the owning
  manager and shared value checks in `models`. Leave queries, locks, and
  transaction mechanics behind a `database` contract.
- Get an explicit owner decision before adding a layer, changing dependency
  rules, or introducing a shared test-helper package. Update the architecture
  decision and package documentation together.
