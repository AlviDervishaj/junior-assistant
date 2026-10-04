# Domain Docs

This repository uses a single-context domain-doc layout.

## Before exploring

- Read `GLOSSARY.md` when it exists.
- Read relevant decisions under `docs/adr/` when they exist.
- If these files are absent, proceed silently. Create them only when actual terminology or architectural decisions need recording.

## Layout

```
/
├── GLOSSARY.md
├── docs/adr/
└── src/
```

## Vocabulary

Use terms defined in `GLOSSARY.md`. If a needed concept is missing, reconsider whether the term is necessary or note the gap for domain modeling.

## ADR conflicts

If proposed work contradicts an existing ADR, surface the conflict explicitly instead of silently overriding it.
