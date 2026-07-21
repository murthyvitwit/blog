# Murthy

Personal essay site. Write Markdown in git; a small Go program builds static HTML.

Default Pages URL: https://murthyvitwit.github.io/blog/  
Custom domain: https://murthy.xyz/ (`domain` in [`config.yaml`](config.yaml)).

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

Output lands in `site/`. If `domain` is set, build also writes `site/CNAME`.

## Deploy (GitHub Pages)

1. In the repo: **Settings → Pages → Source: GitHub Actions**.
2. Push to `main`. The workflow builds the site and deploys `site/`.

## Custom domain

This site uses the apex domain **murthy.xyz** (`domain` in [`config.yaml`](config.yaml)). Build writes `site/CNAME` automatically.

1. Push to `main` so the CNAME deploys with Pages.

2. At your DNS provider for `murthy.xyz`, add GitHub Pages A/AAAA records:

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

   Optional: point `www.murthy.xyz` with a `CNAME` → `murthyvitwit.github.io`, then add `www.murthy.xyz` as a Pages domain alias if you want www as well.

3. In the repo: **Settings → Pages → Custom domain** — enter `murthy.xyz`, wait for DNS check, then enable **Enforce HTTPS**.

DNS/TLS often completes in minutes; it can take up to about 24 hours.
