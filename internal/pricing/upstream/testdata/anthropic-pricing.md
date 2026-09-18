---
title: Pricing
url: https://platform.claude.com/docs/en/about-claude/pricing
description: Learn about Anthropic's pricing structure for models and features
---

This page provides detailed pricing information for Anthropic's models and features. All prices are in USD.

## Model pricing

The following table shows pricing for all Claude models:

| Model                                                                                                                                 | Base input tokens | 5m cache writes | 1h cache writes | Cache hits and refreshes | Output tokens |
| :------------------------------------------------------------------------------------------------------------------------------------ | :---------------- | :-------------- | :-------------- | :----------------------- | :------------ |
| Claude Fable 5.1                                                                                                                      | $10 / MTok        | $12.50 / MTok   | $20 / MTok      | $0.25 / MTok<sup>1</sup> | $50 / MTok    |
| Claude Mythos 5.1 ([limited availability](https://anthropic.com/glasswing))                                                           | $10 / MTok        | $12.50 / MTok   | $20 / MTok      | $0.25 / MTok<sup>1</sup> | $50 / MTok    |
| Claude Fable 5                                                                                                                        | $10 / MTok        | $12.50 / MTok   | $20 / MTok      | $1 / MTok                | $50 / MTok    |
| Claude Mythos 5 ([limited availability](https://anthropic.com/glasswing))                                                             | $10 / MTok        | $12.50 / MTok   | $20 / MTok      | $1 / MTok                | $50 / MTok    |
| Claude Opus 5                                                                                                                         | $5 / MTok         | $6.25 / MTok    | $10 / MTok      | $0.50 / MTok             | $25 / MTok    |
| Claude Opus 4.8                                                                                                                       | $5 / MTok         | $6.25 / MTok    | $10 / MTok      | $0.50 / MTok             | $25 / MTok    |
| Claude Opus 4.5                                                                                                                       | $5 / MTok         | $6.25 / MTok    | $10 / MTok      | $0.50 / MTok             | $25 / MTok    |
| Claude Opus 4.1 ([retired, except on Bedrock and Google Cloud](https://platform.claude.com/docs/en/about-claude/model-deprecations))  | $15 / MTok        | $18.75 / MTok   | $30 / MTok      | $1.50 / MTok             | $75 / MTok    |
| Claude Sonnet 5                                                                                                                       | $2 / MTok         | $2.50 / MTok    | $4 / MTok       | $0.20 / MTok             | $10 / MTok    |
| Claude Sonnet 4.6                                                                                                                     | $3 / MTok         | $3.75 / MTok    | $6 / MTok       | $0.30 / MTok             | $15 / MTok    |
| Claude Haiku 4.5                                                                                                                      | $1 / MTok         | $1.25 / MTok    | $2 / MTok       | $0.10 / MTok             | $5 / MTok     |
| Claude Haiku 3.5 ([retired, except on Bedrock and Google Cloud](https://platform.claude.com/docs/en/about-claude/model-deprecations)) | $0.80 / MTok      | $1 / MTok       | $1.60 / MTok    | $0.08 / MTok             | $4 / MTok     |

*<sup>1 Cache hits and refreshes on Claude Fable 5.1 and Claude Mythos 5.1 are priced at 0.025x the base input price. All other models use the standard 0.1x multiplier.</sup>*

<Note>
  MTok = Million tokens.
</Note>

### Batch processing

The Batch API allows asynchronous processing of large volumes of requests with a 50% discount on both input and output tokens.

| Model            | Batch input  | Batch output  |
| :--------------- | :----------- | :------------ |
| Claude Fable 5.1 | $5 / MTok    | $25 / MTok    |
| Claude Opus 5    | $2.50 / MTok | $12.50 / MTok |
| Claude Sonnet 5  | $1 / MTok    | $5 / MTok     |
