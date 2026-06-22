---
name: summarize
description: Summarize or extract text from URLs, local files, and web content.
---

# Summarize

Summarization and text extraction capabilities built into NeoRay.

## When to Use

Use this skill when the user asks to:
- Summarize an article, document, or web page
- Extract text from a URL or file
- Provide a concise overview of content
- "What's this link about?"
- "Give me a summary of this"

## Quick Start

For local files:
1. Use `read_file` to load the content
2. Ask the LLM to summarize it in the conversation

For URLs:
1. Use the `web_fetch` tool to get the content
2. Ask the LLM to summarize the fetched content

## Summarization Strategies

### Short Summary (1-2 paragraphs)
Focus on:
- Main topic or thesis
- Key conclusions or findings
- Critical context needed to understand the content

### Medium Summary (3-4 paragraphs)
Include:
- Short summary points
- Key supporting arguments or evidence
- Important details or examples
- Context about the source or author

### Long Summary (detailed)
Cover:
- Full structure and flow
- All major sections and their content
- Key data points or statistics
- Nuances and caveats
- Counterarguments or alternative perspectives

## Best Practices

1. **Adapt to the source**: Technical content needs different treatment than casual writing
2. **Preserve accuracy**: Don't introduce new information or change the original meaning
3. **Highlight structure**: Show how the content is organized
4. **Note limitations**: If the content is incomplete or has biases, mention that
5. **Ask for clarification**: If the user doesn't specify summary length, choose medium as default and offer to expand or shorten