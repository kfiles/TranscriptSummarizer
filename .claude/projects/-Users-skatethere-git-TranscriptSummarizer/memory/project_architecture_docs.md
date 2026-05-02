---
name: Architecture documentation
description: Full C4 architecture docs created in docs/architecture/; PlantUML C4 diagrams, Mermaid sequence diagrams, resource inventory
type: project
---

Architecture documentation suite written to docs/architecture/ (6 files, ~1761 lines total).

**Why:** User requested C4 architecture diagrams + cloud resource inventory for developer onboarding and operational reference.

**Files created:**
- OVERVIEW.md — narrative, design decisions, admin tools, doc map
- c1-context.md — PlantUML C4 Context; actors, external systems, relationships
- c2-containers.md — PlantUML C4 Container; all GCP/Firebase resources + admin tools
- c3-components.md — PlantUML C4 Component; all Go packages inside the Cloud Function
- c4-code.md — Mermaid sequence diagrams for 5 flows: webhook→transcription→rebuild, playlist sync (admin + runtime), officials retrieval (admin + runtime), subscription renewal, Cloud Build pipeline
- cloud-resources.md — full resource inventory, IAM table, secrets table, env vars reference, operational commands

**How to apply:** Reference these docs for any work touching infra, IAM, or the pipeline flow. PlantUML renders in VS Code with the PlantUML extension. Mermaid renders natively in GitHub.
