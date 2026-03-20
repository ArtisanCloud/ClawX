---
name: web-search
description: Search the public web for recent information and return concise, source-linked results.
aliases:
  - search
  - web
  - internet-search
---
# web-search

Use this skill when the user asks to search the web, find latest updates, compare recent news, or collect source links.

## Inputs
- `query` (required): user search query
- `recency_days` (optional): prefer recent results
- `domains` (optional): domain allowlist

## Outputs
- A ranked result list with title, snippet, url, and publish date when available.
- A short summary with source links.

## Safety
- Prefer authoritative primary sources.
- For high-stakes topics (medical/legal/financial), require explicit source links.
