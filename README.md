# Murthy

Personal essay site. Write Markdown in git; a small Go program builds static HTML.

Live site (after Pages is enabled): https://murthyvitwit.github.io/blog/

## Write an essay

1. Add a file under `posts/`, e.g. `posts/my-essay.md`:

```markdown
---
title: My essay title
date: 2026-07-21
---

Your writing here.
```

2. The filename (without `.md`) becomes the URL slug: `my-essay.html`.
3. Rebuild locally or push to `main` to publish.

Site name and bio live in [`config.yaml`](config.yaml).

## Local preview

```bash
go run . serve
```

Open http://localhost:8000

Build only (no server):

```bash
go run . build
```

Output lands in `site/`.

## Deploy

Push to `main`. GitHub Actions builds the site and deploys to GitHub Pages.

One-time setup in the repo: **Settings → Pages → Source: GitHub Actions**.
