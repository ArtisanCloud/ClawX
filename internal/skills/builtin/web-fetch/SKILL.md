---
name: web-fetch
description: Fetch and parse a specific webpage or document URL for structured extraction and summarization.
aliases:
  - fetch
  - url-fetch
  - page-read
---
# web-fetch

Use this skill when the user provides a URL and asks to extract facts, summarize content, or verify a claim.

## Inputs
- `url` (required): target URL
- `selectors` (optional): extraction hints
- `max_chars` (optional): output size bound

## Outputs
- Normalized content blocks and key facts.
- Optional summary and cited source URL.

## Safety
- Respect robots/external policy constraints enforced by runtime.
- Report fetch failures with actionable diagnostics.
