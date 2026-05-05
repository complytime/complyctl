---
pack_id: content
language: Any
version: 1.0.0
---
<!-- scaffolded by uf vdev -->

# Convention Pack: Content (Documentation, Blog, PR/Comms)

This is the content convention pack for the Unbound Force
agent ecosystem. It contains writing standards for all
content-producing agents: The Scribe (technical docs),
The Herald (blog/announcements), and The Envoy
(PR/comms). It complements the language-agnostic
`default.md` pack with content-specific standards.

When this pack is active, content agents load it
alongside `default.md`. Each agent focuses on the
sections relevant to its domain:

- **Scribe** → VB + TD + FA + FT sections
- **Herald** → VB + BA + FA + FT sections
- **Envoy** → VB + PR + FA + FT sections

---

## Voice & Brand (VB) — All Content Agents

- **VB-001** [MUST] Content MUST use active voice as the
  default. "Gaze detects side effects" not "Side effects
  are detected by Gaze." Passive voice is acceptable
  when the actor is genuinely unknown or unimportant.

- **VB-002** [MUST] Content MUST NOT use weasel words
  that dismiss the reader's effort: "simply," "just,"
  "easily," "obviously," "of course." These words
  signal that the writer has not considered the reader's
  perspective.

- **VB-003** [MUST] Terminology MUST be consistent
  across all content. The same concept MUST use the same
  term everywhere. If a concept has multiple common
  names, choose one canonical term and use it
  consistently. Introduce the canonical term on first
  use with any necessary context.

- **VB-004** [MUST] Content MUST reflect the project's
  current state accurately. Never present aspirational
  capabilities as current features. Use "Current
  Limitations" sections to honestly scope the tool's
  state. Version numbers and limitation sections signal
  maturity level.

- **VB-005** [MUST] Content MUST NOT use loaded AI
  terminology ("AGI," "intelligent," "self-improving,"
  "sentient") without precise definition and
  justification. Describe what the tool does, not what
  it aspirationally is.

- **VB-006** [SHOULD] The brand voice SHOULD be direct,
  technically credible, and community-minded. Frame
  tools as amplifiers of human intent, not autonomous
  replacements.

- **VB-007** [SHOULD] Content SHOULD pass the "so what"
  test: the benefit to the reader SHOULD be explicit,
  not implied. "Gaze detects side effects" is a feature.
  "Gaze tells you which tests are actually verifying
  behavior" is a benefit.

---

## Technical Documentation (TD) — The Scribe

- **TD-001** [MUST] Technical documentation MUST be
  task-oriented — it answers "how do I do X?" before
  "how does X work internally?" Prefer imperative
  instructions ("Run `uf setup`") over passive
  descriptions ("The setup process can be initiated").

- **TD-002** [MUST] All code examples and shell commands
  MUST be copy-pasteable and work when executed directly.
  Test commands before including them.

- **TD-003** [MUST] Documentation MUST NOT contain
  placeholder text, TODO markers, "Coming Soon," "TBD,"
  or empty sections. Only publish pages with real,
  complete content.

- **TD-004** [MUST] Every page or section MUST establish
  what it covers and why the reader should care in the
  first paragraph. A reader who stops after the opening
  MUST understand the purpose.

- **TD-005** [SHOULD] Documentation SHOULD progress from
  general to specific: concept overview, then practical
  usage, then reference details, then edge cases. The
  most common reader task SHOULD be addressable without
  scrolling past uncommon content.

- **TD-006** [SHOULD] Technical jargon, acronyms, and
  project-specific terms SHOULD be explained on first
  use or linked to a page that explains them. Assume the
  reader is a mid-level developer encountering the
  project for the first time.

- **TD-007** [MUST] API documentation (Go packages,
  CLI commands) MUST document parameters, return values,
  error conditions, and side effects. GoDoc-style
  comments MUST start with the identifier name.

- **TD-008** [SHOULD] Every documentation page or
  section SHOULD end with cross-links to related
  content. No page is a dead end.

---

## Blog & Announcements (BA) — The Herald

- **BA-001** [MUST] Blog posts MUST have a narrative arc:
  problem statement, approach, evidence or walkthrough,
  and a conclusion with a call to action. Posts that are
  lists of features without context do not engage
  readers.

- **BA-002** [MUST] Blog post titles MUST be specific and
  descriptive. "Update on Progress" is too vague. Titles
  MUST communicate both the topic and the value
  proposition.

- **BA-003** [SHOULD] Blog posts SHOULD include
  real-world examples, actual output, or concrete data
  rather than abstract descriptions. Show, then explain.

- **BA-004** [MUST] Blog posts MUST NOT contain
  time-sensitive language that becomes stale ("recently,"
  "new," "just launched," "this week") unless absolutely
  necessary. Use specific dates if temporality matters.

- **BA-005** [SHOULD] Blog posts SHOULD be self-contained
  — a reader arriving from search or social media SHOULD
  understand the post without reading other content.
  Link to detailed docs for reference but do not require
  them for comprehension.

- **BA-006** [MUST] Release notes MUST group changes by
  user impact: features, fixes, improvements, breaking
  changes. Each entry MUST explain the user benefit, not
  just what code changed.

- **BA-007** [MUST] Announcements MUST lead with why the
  change matters to the reader, not what the team built.
  "You can now set up in 12 steps instead of 15" is
  stronger than "We refactored the setup pipeline."

---

## Public Relations (PR) — The Envoy

- **PR-001** [MUST] External communications MUST
  maintain a consistent brand voice across all channels.
  Every piece should feel like it comes from the same
  team.

- **PR-002** [MUST] Press releases MUST lead with the
  most newsworthy angle. Structure: headline, dateline,
  lead paragraph (who/what/when/where/why), supporting
  details, boilerplate.

- **PR-003** [SHOULD] Social media content SHOULD be
  adapted for the target platform's norms and character
  limits. Twitter/X requires brevity; LinkedIn allows
  professional depth; GitHub Discussions allows
  technical detail.

- **PR-004** [MUST] Every external communication MUST
  include a clear call to action: try it, read more,
  star the repo, join the discussion.

- **PR-005** [SHOULD] Community updates SHOULD
  acknowledge community contributions (PRs, issues,
  discussions) by name. The community is a partner,
  not an audience.

- **PR-006** [MUST] External communications MUST NOT
  announce unbuilt features. Every claim must trace back
  to a verified, shipped capability.

- **PR-007** [SHOULD] Key message discipline: each
  communication SHOULD reinforce 1-2 core messages, not
  try to cover everything. Focus is more effective than
  comprehensiveness.

---

## Fact-Checking & Accuracy (FA) — All Content Agents

- **FA-001** [MUST] All factual claims (metrics,
  capabilities, file counts, version numbers) MUST be
  verified against the actual tool's current state.
  Never fabricate features, overstate maturity, or
  present aspirational capabilities as current
  functionality.

- **FA-002** [MUST] Numerical claims (counts,
  percentages, scores) MUST be consistent across all
  content that references them. If accuracy is "84.7%"
  in one place, it MUST be "84.7%" everywhere.

- **FA-003** [SHOULD] When researching upstream tool
  behavior, feature capabilities, or cross-repo context
  for accuracy, agents SHOULD use Dewey MCP tools as the
  primary discovery mechanism. Dewey indexes all sibling
  repositories and the org workspace.

- **FA-004** [MUST] After discovering relevant content
  via Dewey, agents MUST use the Read tool for targeted
  file access to confirm exact numbers, flags, and
  capabilities before writing.

---

## Formatting (FT) — All Content Agents

- **FT-001** [MUST] Use fenced code blocks (triple
  backticks) with a language identifier for all code
  examples: ` ```bash `, ` ```yaml `, ` ```json `,
  ` ```go `, etc.

- **FT-002** [MUST] Use inline code spans (single
  backticks) for commands (`uf setup`), filenames
  (`spec.md`), flags (`--verbose`), and identifiers.

- **FT-003** [MUST] Markdown tables MUST have aligned
  pipes, header separators with at least 3 dashes per
  column, and spaces around cell content.

- **FT-004** [SHOULD] Prose paragraphs SHOULD be 3-5
  sentences. Blocks of text longer than 5 sentences
  without a visual break SHOULD be split.

- **FT-005** [SHOULD] Lists SHOULD use consistent
  markers within a section: either all `-` or all `*`
  or all numbered.

- **FT-006** [MUST] Headings MUST follow a strict
  hierarchy — no skipping levels (e.g., H2 to H4
  without H3).

---

## Custom Rules

<!-- This section is intentionally empty in the canonical
     pack. Project-specific custom rules belong in
     content-custom.md alongside this file. Custom rules
     use the CR-NNN identifier prefix. -->
