# Murthy

Personal essay site. Write Markdown in git; a small Go program builds static HTML.

Live site: https://murthy.xyz/

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

Site name, bio, and domain live in [`config.yaml`](config.yaml).

## Local preview

```bash
go run . serve
```

Open http://localhost:8000

Build only (no server):

```bash
go run . build
```

Output lands in `site/` (gitignored). If `domain` is set, build also writes `site/CNAME`.

## Deploy (GitHub Pages)

Push to `main`. The **Deploy site** workflow:

1. Builds HTML from `posts/`
2. Publishes **only** that HTML to the `gh-pages` branch (so the site root is your essays, not `/site/`)
3. Points Pages at `gh-pages` / `/`

If the root URL still shows the README, set manually once:

**Settings → Pages → Deploy from a branch → Branch: `gh-pages` → Folder: `/ (root)`**

Do **not** choose “Deploy from a branch → `main` → `/`” (that publishes the whole repo under `/site/`).  
Do **not** use the suggested “Static HTML” starter workflow.

## Custom domain

Domain: **murthy.xyz** (`domain` in [`config.yaml`](config.yaml)).

1. At GoDaddy (or your DNS), apex A/AAAA records:

   | Type | Value |
   |------|--------|
   | A | `185.199.108.153` |
   | A | `185.199.109.153` |
   | A | `185.199.110.153` |
   | A | `185.199.111.153` |
   | AAAA | `2606:50c0:8000::153` |
   | AAAA | `2606:50c0:8001::153` |
   | AAAA | `2606:50c0:8002::153` |
   | AAAA | `2606:50c0:8003::153` |

   Optional: `www` CNAME → `murthyvitwit.github.io`

2. **Settings → Pages → Custom domain** → `murthy.xyz` → **Enforce HTTPS**
