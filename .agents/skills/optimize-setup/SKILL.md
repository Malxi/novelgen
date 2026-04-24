# optimize-setup

Optimize the setup module outputs and prompts.

Priority order: Quality > Speed > Cost.

Goals
- Improve correctness and completeness of story setup data.
- Reduce ambiguity and internal contradictions.
- Preserve output format and required fields.

Guidelines
- Inspect setup flow before proposing changes.
- Prefer minimal, targeted edits that improve output quality.
- Do not add new fields unless required by existing downstream logic.
- Record concrete improvements in setup_improvements.md (append "- improve:" lines).

Deliverables
- A short list of proposed changes and rationale.
- A minimal diff implementing the highest-impact quality improvements.
