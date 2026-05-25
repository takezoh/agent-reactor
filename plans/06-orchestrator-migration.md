# 06-orchestrator-migration

harness/ ディレクトリを orchestrator/ にリネームする計画。

## 概要

Symphony 実装全体のディレクトリ構造を整理し、SPEC §3.1 に準拠させるため、`harness/` を `orchestrator/` に変更する。

## 構造変更

orchestrator/ に以下のパッケージを再構成する：

- orchestrator/workflowfile/
- orchestrator/wfconfig/
- orchestrator/tracker/
- orchestrator/scheduler/ (= SPEC §3.1.4 Orchestrator)
- orchestrator/workspace/
- orchestrator/agent/
- orchestrator/httpserver/
- orchestrator/prompt/
- orchestrator/metrics/

## バイナリ名の変更

- cmd/harness/ → cmd/orchestrator/

## 用語定義

- orchestrator (service): Symphony 実装全体。バイナリ名・ディレクトリ名
- scheduler: SPEC §3.1.4 が定義する scheduling brain。実装は orchestrator/scheduler/
