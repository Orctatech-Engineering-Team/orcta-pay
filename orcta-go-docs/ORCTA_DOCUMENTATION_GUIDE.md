# Orcta Documentation Guide

A document exists to move a reader from "I don't know" to "I can act" as fast as possible. That's the whole policy.

Everything below is what that means in practice.

---

## 1. What we take from each, and what we leave

### Swift API docs

Swift's guideline says: weight almost everything on the summary line. A doc comment is a single sentence fragment, ending in a period. If you can't summarize an API's behavior in one clear fragment, the API itself may be designed wrong.

### Basecamp (Shape Up)

Shape Up's pitch structure has five ingredients: problem, appetite, solution, rabbit holes, no-gos. Appetite, fixing a time budget *before* shaping the solution, shuts down the "there's always a better solution" spiral before it starts.

### Uber

Two habits from Uber's technical writers: write with a specific reader's pain points in mind, and cut roundabout phrasing on sight. Give a long doc a TL;DR at the top so a reader can find their answer without reading the whole thing.

### The throughline

The writer did the thinking so the reader doesn't have to re-derive it.

---

## 2. Core principles

1. **Say the point first, then justify it.** Inverted pyramid, not a build-up. The first sentence of a section should be true even if the reader stops there.

2. **Write for the reader about to make a decision**, not the reader who already knows the answer. If you're explaining `payments` to someone who'll never touch it, you're writing the wrong doc for the wrong audience.

3. **One idea per sentence. One canonical example per concept.** Three half-examples are worse than one complete one: a reader has to reconcile them; with one, they just read it.

4. **Every rule carries its reason, next to it.** A rule without a reason gets silently ignored the first time it's inconvenient. "Don't use float for money" is a suggestion. "Don't use float for money, because rounding errors are silent until a reconciliation report doesn't add up" is a rule someone remembers under pressure.

5. **Table over prose when comparing options**, listing tradeoffs, or defining a closed set. Prose for reasoning, narrative, and walking through a flow.

6. **No hedging.** "It might be worth considering possibly using a mutex here" is not a sentence anyone should ship. Say "use a mutex here" and put the reasoning in the next sentence.

7. **Active voice, present tense.** "The service validates the token," not "the token should be validated by the service."

8. **Short paragraphs, 2 to 4 sentences, one idea.** If a paragraph needs a fifth sentence, it probably wants to be two paragraphs or a table.

9. **Concrete over abstract.** Real field names, real error strings, real numbers. "An error occurs" tells you nothing you can act on; "returns `storage.ErrNotFound`" does.

10. **Every doc states its status and owner.** A design doc with no date and no "who do I ask" is a doc nobody trusts six months from now.

11. **Every sentence earns its keep.** Read a sentence, then read the paragraph without it. If nothing is lost, cut it.

---

## 3. Voice, with before/after

See it applied, not just stated.

**Hedging → direct**
> Before: *"It might be a good idea to possibly consider adding a retry here."*
> After: *"Add a retry here. Hubtel's API returns 5xx under load roughly 2% of the time — a single retry with backoff resolves nearly all of those."*

**Passive → active, agency restored**
> Before: *"Errors should be wrapped with context by the caller as they cross package boundaries."*
> After: *"Wrap errors with `%w` as they cross package boundaries."*

**Buried conclusion → front-loaded**
> Before: *"Given the volume of location updates, the cost implications of full-fidelity logging, and the fact that most updates are uneventful, we have decided that..."*
> After: *"Sample successful location-update logs at 1%."*

**Vague → concrete**
> Before: *"Leverage the observability stack to gain visibility into system behavior."*
> After: *"Every order transition emits one wide event with `order_id`, `rider_id`, and `time_to_confirm_ms` — query it in Grafana by any of those fields."*

**Rule with no reason → rule that survives contact with a deadline**
> Before: *"Don't create a generic `admin` package."*
> After: *"Don't give `admin` a package until it hides a real policy decision. Otherwise it's a route group with elevated authz, and giving it a package invites logic to accumulate there that belongs elsewhere."*

**Wordy → said once**
> Before: *"It's really quite important to basically make sure you actually validate the input before you go ahead and process it."*
> After: *"Validate input before processing it."*
>
> Cutting roundabout phrasing doesn't just shorten a sentence. It removes the reader's work of figuring out which words were load-bearing.

---

## 4. Markdown formatting

Formatting isn't decoration. It's part of communicating unambiguously. `Order` the type and "order" the everyday noun look identical without a code span.

- **Headers.** One `#` per document, the title, in Title Case. Every header below that is sentence case. Never skip a level (`##` straight to `####`).
- **Bold.** Reserved for a term's first defining use, or a genuinely load-bearing warning, not decoration on every rule's opening phrase.
- **Code spans.** Any identifier, type, filename, command, or literal value gets backticks. No exceptions. This is the single highest-leverage formatting rule for avoiding ambiguity.
- **Code blocks.** Always fenced with a language tag. One complete block per concept.
- **Tables.** Used only for genuinely comparative or enumerable content, not as a way to make a list look more official.
- **Lists.** Ordered only for genuine sequence. Don't nest past two levels.
- **Line wrapping.** Don't manually wrap lines in documentation. Let the renderer handle it. Manual wrapping at 80 or 100 characters creates artificial line breaks that hurt diff readability when the content changes.
- **Whitespace.** One blank line between a header and its content, one between sections, no trailing whitespace on a line.

## 5. Naming conventions

- **File naming.** `SCREAMING_SNAKE_CASE.md` at the repo root for canonical, cross-team reference docs, the ones everyone is expected to know exist (`CONVENTIONS.md`, `ARCHITECTURE.md`, this guide). `kebab-case.md` for narrower, topic-specific docs living under `docs/` (`docs/payments-ledger-design.md`). ADRs, RFCs, and specs get numbered prefixes (`docs/adr/0001-chi-router.md`, `docs/rfc/0001-broadcast-window-thresholds.md`, `docs/spec/0001-order-api-contract.md`); the number gives a stable reference even if the title changes later. Runbooks are plain kebab-case under `docs/runbooks/` because nobody needs to sort them by number at 2am. Postmortems are date-prefixed under `docs/postmortems/` (`docs/postmortems/2026-07-15-payment-timeout.md`) because the date is the natural lookup key for incident history.
- **Header capitalization.** Sentence case everywhere except the H1 title. Title Case forces a capitalization decision on every word, every time, for no reader-visible benefit. Stated once here in §5, applied consistently.
- **Terminology.** One term per concept, never swapped for variety. Use `rider`, never `driver`. Use `vendor`, never `merchant` or `restaurant`. Use `order`, never `delivery` or `job`. This isn't pedantry. The observability reference material an earlier version of the design doc drew from used `driver` throughout, and every instance was renamed to `rider` before it landed in `ARCHITECTURE.md`. Swapping terms doesn't read as variety; it reads as two different things.

---

## 6. Documentation taxonomy — when to write which

Every doc type exists because a different situation needs a different shape. Using the wrong one wastes the writer's time and the reader's. Every doc starts with YAML frontmatter (`---` delimited) containing machine-parseable metadata: status, owner, created date, relationships. Agents use this to filter, discover, and link docs without parsing prose. The registry at `docs/INDEX.md` lists all docs by type.

### Decision matrix

| Type | Goal | Lifecycle | Header fields | Write it when | Skip it when |
|---|---|---|---|---|---|
| **Design doc** | Explain a system's architecture | Living, updated as system evolves | Status | Building a new domain or major subsystem | A single endpoint or bug fix |
| **Pitch** | Propose what to build, get a bet made | One-shot, becomes ADR when accepted | — | New feature/service that needs a time budget | The decision is already made |
| **ADR** | Record *why this and not that* | Immutable once accepted | Status, Date, Owner, Impact, Supersedes | Every architectural decision, backfill old ones | Small implementation details |
| **RFC** | Get feedback on *how* before deciding | Open discussion, becomes ADR when settled | Status, Created, Owner, Supersedes, Depends on | Cross-team or breaking changes with real alternatives | One option is clearly best |
| **Spec** | Define exact behavior contract | Living, versioned with the feature | Status, Created, Owner, Impact, Applies to | API contracts, payment provider interfaces | Internal package design |
| **Runbook** | Step-by-step recovery procedures | Updated after every incident that uses it | Last updated, Severity, Owner | After you've fixed the same thing twice | Before the system is in production |
| **Postmortem** | Blameless incident review | Write once, reference in runbooks | Date, Severity, Author, Status | After every incident or near-miss | Never skip this |

### The distinctions people get wrong

**RFC vs Pitch.** A Pitch proposes *whether* to build something; it needs a time budget. An RFC proposes *how*; the problem is settled, the approach is not. A Pitch survives by being small enough to bet on. An RFC survives by being thorough enough to review.

**RFC vs Design doc.** A design doc explains a system that exists. An RFC is a proposal under discussion; it invites disagreement. When the RFC lands, it becomes an ADR or gets folded into a design doc.

**Spec vs Design doc.** A design doc explains *why*. A spec defines *what*: exact interfaces, exact error codes, exact edge cases. A design doc can hand-wave. A spec cannot.

**ADR vs Runbook.** An ADR explains why you chose chi over gin. A runbook explains what to do when Hubtel's webhook fails at 2am. Architecture vs operations.

### Startup guidance — what to write first

You have three engineers and a product to ship. Don't write documents nobody reads. Prioritize in this order:

1. **ADRs** — 15 minutes each, highest leverage. One per decision.
2. **Runbooks** — write after the second time you fix something at 2am. The first time you learn. The second time you waste time re-learning.
3. **Postmortems** — after every incident, even a 10-minute outage. The habit matters more than the document length.
4. **Specs** — only for integration boundaries where ambiguity causes bugs. The order API. The Hubtel/Paystack interface. The dispatch protocol.
5. **RFCs** — only when multiple engineers need to agree before a choice is made. Most decisions at your stage are small enough for an ADR.
6. **Design docs** — only for complex new systems. Your existing `ARCHITECTURE.md` is the template.
7. **Pitches** — when you need a time budget before shaping the solution.

Most decisions at startup stage are small enough for an ADR. If you're writing a 5-page document, ask whether it's really a design doc or an RFC.

---

## 7. Structure

**Headers are questions or specific nouns, not filler.** "Observability," "Money Representation," "Provider Selection," not "Overview," "Introduction," "Miscellaneous." A header should tell the reader whether this section has their answer without them reading it.

**The first sentence of a section does the work.** If someone reads only that sentence, they should walk away with the section's actual point, even if they miss the supporting detail.

**Code examples are complete**, not fragments requiring mental reassembly. Comments in code explain *why*, never *what*. The code already says what it does; a comment repeating that is noise.

**Open questions are called out explicitly**, never buried in a paragraph where a reader can skim past them:

> **Open question:** does a shortfall net against tomorrow's payout or trigger a separate collection flow? Needs an answer before the ledger logic is finalized.

Not:

> *"...and it's worth considering how shortfalls might be handled, which is something we haven't fully worked out yet, though there are a few options..."*

**Decision records are short and separate from design docs.** A design doc explains a system; a decision record explains *why this and not that*, at the moment the choice was made, so it doesn't need re-litigating every time someone reads the design doc fresh. See the ADR template below.

---

## 8. Anti-patterns — leave these at the door

- **Throat-clearing.** "It should be noted that," "it is important to understand that," "as previously mentioned." Delete these; the sentence after them loses nothing.
- **Wall of text.** No headers, no lists, no tables, a design decision buried in paragraph four of an undifferentiated block.
- **Over-qualifying an absolute rule.** "Generally speaking, in most cases, secrets shouldn't be logged." Either it's a rule (say it plainly) or it's situational (say what the situation is).
- **How without why.** Documenting the exact steps of a migration without saying what problem it solves is a script, not a document. A reader can't adapt a script to a situation slightly different from the one it was written for.
- **Boilerplate section headers that add zero information.** "Overview," "Conclusion," "Summary" on a two-paragraph doc. If the whole doc is the overview, it doesn't need one.
- **Passive voice that hides who's responsible.** "Mistakes were made" is the canonical example for a reason. In a technical doc, "the retry logic was not implemented" should be "we didn't implement retry logic," because the next question is *who does, and by when.*
- **Bare identifiers.** Writing `order` or `payments` in running prose with no code span, leaving the reader to guess whether you mean the everyday word or the type/package. If it's a symbol, it gets backticks, every time.
- **Synonym-swapping a term for variety.** Calling the same thing `rider` in one paragraph and `driver` in the next reads as two different things, not as elegant prose. See §5.

---

## 9. Templates

Standalone template files live in `docs/templates/`. Copy one and fill it in. Every template starts with YAML frontmatter; agents parse this to filter, discover, and link docs. Don't skip it.

### ADR (Architecture Decision Record)
One screen, not a design doc. Short enough that a new hire reads the whole thing without skimming.

```
---
title: ADR-00N
description: <short, specific title>
status: proposed
date: YYYY-MM-DD
owner: <name>
impact: high
supersedes: null
---

## Context
What situation forced a decision. 2-3 sentences, no more.

## Decision
The actual choice, stated as a direct sentence. "We use X."

## Why
The reasoning, and — this is the part usually skipped — what we explicitly gave up by choosing this over the alternative.

## Revisit if
The condition that would make this decision wrong. Not "never," a real trigger: "if order volume exceeds N/day" or "if Hubtel's uptime drops below X%."
```

### Pitch — for proposing what to build, not recording what was decided
Use this when the thing being written about doesn't exist yet and the point is to get a bet made on it: a feature, a new service, a rework. Five ingredients, in order, per Shape Up.

```
---
title: Pitch
description: <name>
---

## Problem
The single specific situation that shows why the status quo doesn't work. Not a category of problem — one concrete story.

## Appetite
How much time this is worth, stated before the solution is designed, not estimated after. "Two weeks" or "six weeks," not "as long as it takes."

## Solution
The core idea, at the level of detail needed to judge the bet — not full implementation detail. Rough sketches/flows are fine here.

## Rabbit Holes
Specific risks or unknowns that could blow the appetite if not addressed now — and how to avoid getting stuck in them.

## No-gos
What's explicitly out of scope, stated on purpose, so nobody assumes it's included by omission.
```

An ADR records a decision that's already been made and needs a durable reason attached. A Pitch proposes a decision that hasn't been made yet and needs to survive being compared against other pitches. Don't use one for the other: a Pitch with no appetite is unshaped work by definition, and an ADR that reads like a sales pitch has buried its actual reasoning.

### RFC (Request for Comments)
Use when the problem is settled but the approach is not, and you need team input before deciding. Not for small decisions; those get an ADR. The "Current state" / "Proposed state" split makes the delta explicit. "Trade-offs and risks" forces honest discussion of your own proposal's downsides.

```
---
title: RFC-00N
description: <title>
status: draft
created: YYYY-MM-DD
owner: <name>
supersedes: null
depends_on: null
---

## Summary
One paragraph. What's broken or missing, and why it needs solving now.

## Current state
What exists today. Concrete, not abstract — real field names, real error modes, real numbers.

## Proposed state
What changes. Enough detail to teach it to a new engineer, not enough to implement. If this is a system change, explain how it interacts with existing components.

## Trade-offs and risks
What's bad about this proposal. Costs, risks, unknowns. Be honest — an RFC that only lists pros has buried its actual reasoning.

## Alternatives considered
| Option | Pros | Cons |
|---|---|---|
| Option A | ... | ... |
| Option B | ... | ... |

## Open questions
- Question 1
```

### Software spec
Use for integration boundaries: API contracts, payment provider interfaces, protocol behavior. Not for internal package design (that's what design docs are for). A spec that leaves ambiguity is an incident waiting to happen.

```
---
title: SPEC-00N
description: <component or interface name>
status: draft
created: YYYY-MM-DD
owner: <name>
impact: high
applies_to: <package/service/interface>
---

## Purpose
One sentence. What this spec covers and why it exists.

## Interface
The exact contract. Types, methods, parameters, return values.

## Behavior
Precise rules. State machines, timing guarantees, idempotency semantics, retry behavior. No ambiguity.

## Edge cases
What happens when requests time out, idempotency keys are reused, or referenced entities are deleted between check and write.

## Out of scope
What this spec intentionally does NOT cover.
```

### Runbook
Written for the on-call person at 2am, not the architect. Every step has a concrete command or query, no "check if things look normal."

```
---
title: Runbook
description: <scenario>
last_updated: YYYY-MM-DD
severity: p1
owner: <team>
---

## Symptoms
What the on-call person sees. Alert names, dashboard links, error messages.

## Diagnosis
Step-by-step: what to check first, what to check second. Each step has a concrete command or query.

## Recovery
Ordered steps to restore service. Each step is a single action with a verification.

## Prevention
What to do after recovery so this doesn't happen again.

## Escalation
Who to page if the steps above don't resolve it.
```

### Postmortem
Blameless. Focus on systems, not people. "Human error" is never the root cause; the system allowed a human action to cause this. What guard was missing?

```
---
title: Postmortem
description: <incident title>
date: YYYY-MM-DD
severity: p1
author: <name>
status: draft
---

## Summary
One paragraph. What happened, impact, resolution.

## Timeline
All times in UTC. Every significant action, observation, and decision.

## Root cause
What actually broke, and why. Not "human error" — the system allowed a human action to cause this. What guard was missing?

## Impact
Users affected, orders lost/delayed, revenue impact, duration.

## What went well
Specific things that worked. Concrete, not platitudes.

## What went poorly
Specific things that didn't. Same standard.

## Action items
| Action | Owner | Due | Issue |
|---|---|---|---|
| ... | @name | YYYY-MM-DD | #123 |

## Lessons learned
One or two sentences. The thing that, if the team remembers it, prevents the next incident of this class.
```

### PR description
Three sections. No fluff. A reviewer should know what changed, why, and how to verify it without reading the diff.
```
---
title: PR description
---

## What
One sentence. What changed.

## Why
The problem this solves — link the issue/conversation if one exists.

## How to verify
Concrete steps a reviewer or future-you can actually run.
```

### README (for a package or service)
Answer four questions: what is this, who owns it, how do I run it, where do I go for more. A README that takes a new engineer from clone to running tests in under ten minutes saves a measurable amount of time every onboarding.
```
---
title: <name>
---

One sentence: what this hides (the Parnas secret), not what it contains.

## Depends on
Interfaces this package declares and expects satisfied — not implementation details of who satisfies them.

## Consumed by
Who calls into this, if known — helps a reader gauge blast radius of a change.
```

---

## 10. Before publishing — checklist

Run through this before sharing any doc. Not a bureaucratic gate, a quick scan for the things that make a reader stop trusting the document.

- [ ] First sentence of the doc states its point, not its topic
- [ ] Every rule has its reason next to it, not implied or absent
- [ ] No hedging phrases survived a re-read (`might`, `could consider`, `it may be worth`)
- [ ] At least one concrete example per non-trivial concept
- [ ] Comparisons are tables, not paragraphs
- [ ] Open questions are called out explicitly, not buried
- [ ] Status and date are present
- [ ] Every identifier, filename, or literal value is in a code span — no bare `order` that should have been `` `Order` ``
- [ ] Headers below the title are sentence case, no level skipped
- [ ] No filler modifiers survived (`very`, `really`, `just`, `simply`, `basically`, `actually`, `quite`)
- [ ] Same term used for the same concept everywhere in the doc — no synonym-swapping for variety
- [ ] Read it out loud once — if a sentence is awkward to say, it's awkward to read

---

*Section 1's claims about Swift, Basecamp, and Uber are checked against their own sources, not asserted from memory: [Swift API Design Guidelines](https://www.swift.org/documentation/api-design-guidelines/), [Shape Up](https://basecamp.com/shapeup/1.5-chapter-06), and Uber's ["Learning on the Go: Engineering Efficiency with Concise Documentation"](https://www.uber.com/en-IN/blog/learning-on-the-go-engineering-efficiency-with-concise-documentation/).*
