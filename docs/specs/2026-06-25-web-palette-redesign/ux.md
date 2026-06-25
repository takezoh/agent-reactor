---
kind: ux
id: ux-2026-06-25-web-palette-redesign
title: Web UI コマンドパレット 再設計 (TUI 移植からの Web 最適化, 案 A 最小)
created: 2026-06-25
status: accepted
owners: [take.gn@gmail.com]
goal: Web UI コマンドパレットを TUI 移植のままから Web UX に最適化し、scope 統合 / disabled visible / chip 3 経路 toggle / hover follow / active context header / push 送信先明示 toast の 6 点を観測可能な振る舞いとして再設計する (案 A 最小)。
target_users:
  - arc Web UI で複数セッション (new-session + push) を頻繁に開閉する個人開発者
  - TUI 版 arc に慣れていて Web 版に移行しつつあるユーザー (worktree / host を頻繁に切り替える)
  - Web 慣れだが TUI を触らないユーザー (Tab / Shift+Tab を知らず pointer 操作主体)
  - アクセシビリティ支援技術 (NVDA / VoiceOver) で操作するキーボードユーザー
primary_flows:
  - id: F-001
    name: パレット起動 → 統合 list から new-session を選び送信 (正常系)
    steps:
      - "ユーザーが TerminalPane / SessionList にフォーカスがある状態で prefix+p (または prefix+C-p) を押す"
      - "CommandPalette が overlay として開き、palette-input にフォーカスが移る (TerminalPane の textarea は blur される)"
      - "palette header に Active context 行が表示される (例: `Active: bar / sess_abcd1234`)。active session が未選択なら icon + `— No active session` を secondary 表示で出す (色だけに依存しない)"
      - "listbox には [New Session, push:save, push:resume, ...] の順で 1 list が描画される。ScopeSegment は描画されない"
      - "ユーザーが `new` とタイプすると fuzzy filter が走り `New Session` 行が先頭 hit になり keyboard cursor がその行に表示される"
      - "ユーザーが Enter を押すと Project listbox に遷移し先頭オプションが選択状態で描画される"
      - "ユーザーが project を選び Enter すると command 入力行に進み、project の isGit / isSandboxed に応じて Worktree / Host chip が入力欄下に常時表示される (chip 左に `[W]` / `[H]` の key hint icon が見える)"
      - "ユーザーが pointer で Worktree chip をクリックすると chip の aria-checked が true になり、入力欄 focus は失われない"
      - "ユーザーが command を入力し Enter を押すと palette は閉じ、新しいセッションが画面上で active になる"
      - "palette が閉じた瞬間、起動前にフォーカスしていた DOM 要素 (TerminalPane など) にフォーカスが戻る"
  - id: F-002
    name: push tool が disabled な状態で list 上に表示・選択ブロック
    steps:
      - "ユーザーが Web UI を開いた直後 (Active context 行に `— No active session` が見える状態) で palette を起動する"
      - "listbox の上段に [New Session]、下段に separator (視覚的横線) を挟んで [push:save (disabled), push:resume (disabled), ...] が並ぶ。disabled 行は warning icon + secondary text `No active session` が同一行に表示される"
      - "ユーザーが `save` とタイプすると `push:save` 行が表示され、行末に同じく warning icon + `No active session` が出る"
      - "ユーザーが Enter または pointer click で disabled 行を選択しようとする"
      - "palette は閉じず、該当行が 1 回 shake / flash し、入力欄直下の inline status 領域に `\"save\" is unavailable: No active session` のメッセージが aria-live=polite で 1 回読み上げられる (toast は出さない)"
      - "ユーザーが Esc で閉じるか、別 tool を選び直すかを判断できる"
  - id: F-003
    name: hover follow と keyboard cursor の単一 highlight
    steps:
      - "ユーザーが palette を開き listbox に 5 行以上見えている状態で、↓ を 3 回押す。4 行目に keyboard cursor highlight (aria-selected=true) が表示される"
      - "ユーザーが pointer を 6 行目 (有効な行) に hover させる"
      - "6 行目に同じ highlight (aria-selected=true) が移り、4 行目の highlight は消える"
      - "ユーザーが ↓ を 1 回押すと 7 行目に highlight が移り、pointer は 6 行目に置いたままでも highlight は cursor を follow する"
      - "ユーザーが pointer を disabled 行に hover させる"
      - "disabled 行には『情報を読みに来た』subtle hover style が出るのみで、aria-selected は元の cursor 位置に残り続ける"
      - "ユーザーが pointer を listbox 外に出すと、最後の keyboard cursor 行の highlight が再表示される (mouseleave で cursor 位置の visual highlight が確実に復元される)"
  - id: F-004
    name: worktree / host chip を pointer + Alt+W / Alt+H + Tab→Space の 3 経路で toggle
    steps:
      - "ユーザーが new-session の command 入力欄にフォーカスがあり、Worktree chip と Host chip が入力欄直下に並んで見える状態 (各 chip 左に `[W]` / `[H]` の key hint icon)"
      - "ユーザーが pointer で Worktree chip をクリックする。chip の aria-checked が true に変わり、入力欄 focus は維持される"
      - "ユーザーが Alt+W を押す。Worktree chip の aria-checked が false に戻り、入力欄 focus は維持される"
      - "ユーザーが Tab を押すと focus が次の interactive 要素 (chip など) に移る (Tab は素直な focus 移動として機能する。chip toggle 専用の Tab / Shift+Tab ショートカットは存在しない)"
      - "focus が Worktree chip にある状態で Space を押すと aria-checked が toggle される"
      - "chip focus 中に Enter を押すと chip toggle が発火する (form submit にはならない。submit は入力欄 focus 時のみ)"
  - id: F-005
    name: paramless push の送信先明示 toast
    steps:
      - "ユーザーが palette を開き、Active context 行に `Active: bar / sess_abcd1234` が表示されている状態で `save` を fuzzy 入力する"
      - "push:save 行が cursor 位置にあり Enter を押す"
      - "palette overlay が閉じる"
      - "画面右下 (toast 領域) に info level の toast `Sent 'save' → bar · sess_abcd1234` が約 2.5 秒間表示される。toast の sessionID 部分は monospace で描画される"
      - "ユーザーが toast に pointer を hover すると、tooltip / title 属性で full project path (例: `/home/dev/foo/bar`) と full session ID が表示される"
      - "undo 操作は提供されない (push は不可逆)"
  - id: F-006
    name: active session の palette open 中切替を flash + aria-live で告知
    steps:
      - "ユーザーが palette を開き、Active context 行に `Active: foo / sess_001aaaa` が見えている状態"
      - "別ウィンドウ / 別 client から view-update が届き、この client が見ている active session が sess_002bbbb に切り替わる"
      - "Active context 行の表示が `Active: bar / sess_002bbbb` に更新され、行全体が約 600ms subtle background flash する"
      - "同時に aria-live=polite 領域から `Active session changed to bar / sess_002bbbb` が 1 回読み上げられる"
      - "これまで disabled だった push:* 行のうち、新 active session で利用可能になったものは warning icon と `No active session` 表示が消え、separator 上段 (有効グループ) に移動して再描画される。該当行も 1 回 flash する"
      - "ユーザーは Esc で閉じるか、現在の cursor 位置のまま Enter で送信するかを判断できる (送信は新 active 宛になる旨が header と該当行 flash で告知済み)"
  - id: F-007
    name: disabled tool は cursor が skip し、有効グループに着地する
    steps:
      - "listbox が [New Session] / separator / [push:save (disabled), push:resume (disabled), push:status (有効)] の順で並んでいる状態"
      - "ユーザーが ↓ を 1 回押す"
      - "cursor が New Session(index 0) から push:status (有効グループ末尾) に飛ぶ。間の disabled 行は keyboard cursor で skip される (ただし list には visible で表示が残る)"
      - "ユーザーが ↑ を 1 回押すと cursor が New Session(index 0) に戻る"
      - "全行が disabled な異常状態 (movable = 空) になった場合、palette 上部の status badge slot に `Loading commands…` または `No commands available` が表示され、Enter は no-op になる (silent failure ではなく明示メッセージで知らせる)"
  - id: F-008
    name: submit in-flight 中に active 切替が起きた場合の凍結
    steps:
      - "ユーザーが push:save の Enter を押し、palette は閉じ始めていないがネットワーク送信中 (status badge slot に `Sending…` spinner が見える) になる"
      - "その送信中に別 client から active session 切替の view-update が届く"
      - "palette UI (Active context 行 / listbox / status badge) は凍結状態で、送信が解決するまで現状の表示を維持する (silent な context shift で表示と送信先がズレることを防ぐ)"
      - "送信が解決した瞬間に palette は閉じ、F-005 と同じ toast が表示される"
      - "切替後の新 active context は palette を次回開いた時に反映される"
acceptance_scenarios:
  - id: UAC-001
    flow_ref: F-001
    given: "ユーザーが Web UI を開き TerminalPane にフォーカスがある状態"
    when: "prefix+p を押す"
    then: "CommandPalette overlay が画面に表示され、入力欄にカーソルが点滅し、最上部に Active context 行が見える"
  - id: UAC-002
    flow_ref: F-001
    given: "active session が未選択 (画面右上に session 表示がない状態) で palette を開いた直後"
    when: "header 行を視認する"
    then: "`— No active session` というテキストが icon 付きで表示され、color のみに依存せず無効状態が認識できる"
  - id: UAC-003
    flow_ref: F-001
    given: "palette が開いていて listbox の先頭 (New Session) に cursor がある状態"
    when: "Enter を押し Project の先頭オプションを Enter で選び command 入力欄に `hello` と入力して Enter を押す"
    then: "palette overlay が画面から消え、新しいセッションが画面に追加表示され、palette 起動前にフォーカスしていた要素にフォーカスが戻る"
  - id: UAC-004
    flow_ref: F-002
    given: "active session が未選択の状態で palette を開いた直後"
    when: "listbox を視認する"
    then: "[New Session] が上、separator (横線) を挟んで [push:save (warning icon + `No active session`), push:resume (同様)] が下に表示されている"
  - id: UAC-005
    flow_ref: F-002
    given: "上記状態で push:save 行に cursor がある (もしくは pointer で hover している)"
    when: "Enter を押す"
    then: "palette は開いたままで、該当行が 1 回 flash し、入力欄直下に `\"save\" is unavailable: No active session` のテキストが表示され screen reader が同じ文言を読み上げる (toast 通知は出ない)"
  - id: UAC-006
    flow_ref: F-003
    given: "palette listbox に有効な行が 7 件並んでいて、keyboard cursor が 4 行目 (aria-selected=true) にある状態"
    when: "ユーザーが pointer で 6 行目 (有効) に hover する"
    then: "6 行目に aria-selected=true の highlight が移動し、4 行目からは highlight が消える"
  - id: UAC-007
    flow_ref: F-003
    given: "keyboard cursor が有効な行に置かれた状態で、ユーザーが listbox 内の disabled 行に pointer を hover した状態"
    when: "Enter を押す"
    then: "発火するのは元の keyboard cursor 行 (有効な行) であり、disabled 行は発火しない (palette は閉じ、もしくは次 phase に進む)"
  - id: UAC-008
    flow_ref: F-004
    given: "command 入力欄にフォーカスがあり Worktree chip (aria-checked=false) が見える状態"
    when: "ユーザーが Worktree chip を pointer click する"
    then: "Worktree chip の aria-checked が true になり、command 入力欄のキャレットが消えずフォーカスが維持されている"
  - id: UAC-009
    flow_ref: F-004
    given: "command 入力欄にフォーカスがあり Worktree chip (aria-checked=false) が見える状態"
    when: "Alt+W を押す"
    then: "Worktree chip の aria-checked が true になる (pointer click と同じ視覚結果)"
  - id: UAC-010
    flow_ref: F-004
    given: "Worktree chip にフォーカスがある (chip が focus ring を持つ) 状態"
    when: "Enter を押す"
    then: "chip の aria-checked が toggle され、palette overlay は閉じない (form submit が発生しない)"
  - id: UAC-011
    flow_ref: F-005
    given: "Active context 行に `Active: bar / sess_abcd1234` と表示されている palette open 状態"
    when: "`save` を入力して Enter を押す"
    then: "palette が閉じ、画面右下に info 色の toast `Sent 'save' → bar · sess_abcd1234` が表示され、sessionID 部分が monospace で読める"
  - id: UAC-012
    flow_ref: F-005
    given: "上記 toast が表示されている状態"
    when: "toast 上に pointer を hover する"
    then: "tooltip でフルプロジェクトパス (例: `/home/dev/foo/bar`) とフルセッション ID が表示される"
  - id: UAC-013
    flow_ref: F-006
    given: "palette が開いていて Active context 行に `Active: foo / sess_001aaaa` が表示されている状態"
    when: "別 client の操作によりこの client の active session が sess_002bbbb に切り替わる"
    then: "Active context 行が `Active: bar / sess_002bbbb` に変わり、視覚的に約 600ms flash し、screen reader が `Active session changed to bar / sess_002bbbb` を 1 回読み上げる"
  - id: UAC-014
    flow_ref: F-006
    given: "上記の active 切替により push:save 行が disabled → 有効に変化した状態"
    when: "listbox を視認する"
    then: "push:save 行から warning icon と `No active session` 文言が消え、separator の上段 (有効グループ) に移動して 1 回 flash している"
  - id: UAC-015
    flow_ref: F-007
    given: "listbox に [New Session, separator, push:save (disabled), push:resume (disabled), push:status (有効)] が並び keyboard cursor が New Session にある状態"
    when: "↓ を 1 回押す"
    then: "keyboard cursor highlight が push:status に表示され、push:save と push:resume には cursor highlight が出ない (ただし両行は list 上に表示されたままで warning icon + `No active session` が読める)"
  - id: UAC-016
    flow_ref: F-007
    given: "sessionConfig がまだ hydrate されておらず list に operable な行が 1 件も無い状態"
    when: "palette を視認する"
    then: "palette 上部の status badge slot に `Loading commands…` テキストが表示され、Enter を押しても何も起きないことがメッセージで明示される (silent ではない)"
  - id: UAC-017
    flow_ref: F-008
    given: "push:save の Enter を押し palette 上部 status badge slot に `Sending…` spinner が見える状態"
    when: "送信解決前に view-update で active session 切替が届く"
    then: "palette の Active context 行 / listbox 表示は変化せず、`Sending…` 表示が継続している"
  - id: UAC-018
    flow_ref: F-008
    given: "上記の送信が解決した瞬間"
    when: "送信完了 response が返る"
    then: "palette が閉じ、F-005 と同じフォーマットの info toast が表示される (送信先は送信開始時の active context のまま)"
states:
  - "toolSelect 状態 (統合 list): 標準 tool が separator 上段、push tool が下段。各 push 行に warning icon + secondary text disabledReason が active session 状態に応じて表示される"
  - "paramSelect 状態 (Worktree / Host chip): chip は role='switch' aria-checked。chip 左に `[W]` / `[H]` の key hint icon が常時可視"
  - "header Active context 行 (常時可視): `Active: <projBase> / <sid8>` または icon + `— No active session`。client-local activeSessionID (ADR-0046) を source とする"
  - "Active context 変化時 flash 状態: 行全体が約 600ms subtle background flash + aria-live=polite で `Active session changed to ...` を 1 回読み上げ"
  - "submitting 状態 (送信中): palette 上部 status badge slot に `Sending…` spinner、listbox aria-disabled=true、Active context / listbox 表示は凍結"
  - "loading 状態 (sessionConfig 未 hydrate): palette 上部 status badge slot に `Loading commands…`、listbox には new-session のみ表示"
  - "empty 状態 (dynamic options 0 件): ParamEmptyState `No projects available` 表示、後続 param と submit を suppress (既存 FR-A4 維持)"
  - "unavailable 状態 (ctx 構築失敗): palette 上部 status badge slot に `Unavailable` 表示、listbox aria-disabled=true、Active context 行は表示しない (整合性のため)"
  - "inline disabled feedback 状態: 入力欄直下の inline status 領域に `\"<label>\" is unavailable: <reason>` を aria-live=polite で 1 回読み上げ、該当行が 1 回 shake / flash (toast は出さない)"
  - "composing 状態 (IME 変換中): Enter / arrow / Ctrl+N/P / Esc / pointer click / Space chip toggle / Alt+W / Alt+H をすべて guard"
  - "stale-tab 状態 (ADR-0046): 一定時間 inactive 後の再 open 時の挙動は既存維持 (本タスクで変えない)"
edge_cases:
  - "disabled tool は keyboard ↑↓ で skip するが list には visible (warning icon + secondary text 付き)。movable な行が 0 件の場合は ↑↓ no-op で、status badge slot に `Loading commands…` または `No commands available` を明示する (silent failure 禁止)"
  - "pointermove で cursor を有効行に移したあと keyboard ↓ を押したら cursor + 1 (現在位置 + 1)。pointer と keyboard は同じ cursor state を 1 ソース共有する。disabled 行への hover は cursor state を更新せず、subtle hover style のみで知覚される (Enter で誤発火しない)"
  - "Stop session lifecycle action は本タスクでは復活させない (e4fd31d で撤去された経緯 + 案 A 最小スコープ + 不可逆操作は確認モーダルが必須で UX を肥大化させるため別 ADR で再検討)"
  - "Tab / Shift+Tab の chip 切替専用キーバインドは撤去。Tab は素の focus trap 内移動として機能。chip toggle は pointer click / Tab→Space / Alt+W (worktree) / Alt+H (host) の 3 経路。chip focus 中の Enter は toggle (submit にはならない)"
  - "global header (palette 外) の active session 表示は本タスク out of scope。palette 内 header のみ表示 (Open Questions に記録)"
  - "1 list の並び順は『有効グループ → separator → disabled グループ』。グループ内は registry 順を維持。recently used 等の sort 機構は案 C 領域として除外 (Open Questions に記録)"
  - "disabled inline 文言は scopeDisabledReason の戻り値そのまま (ADR-0047 single source 維持)。表示時に装飾を加えない。`\"<label>\" is unavailable: <reason>` の `<reason>` 部分は scopeDisabledReason の生戻り値を埋め込むのみで加工しない"
  - "paramless push 送信時 toast に session 切替リンクは入れない (toast は read-only)。送信先 projBase / sid8 を文字列で明示し、tooltip / title で full path / full sessionID を提供する。誤送信時は次回 palette で正しい session を選び直す"
  - "Active context 行の source は client-local activeSessionID (ADR-0046)。daemon-global active ではない。submit in-flight 中に view-update で active が変わっても palette は凍結し、表示と送信先のズレを構造的に防ぐ"
  - "Active context の変化は flash (約 600ms background) + aria-live=polite (`Active session changed to ...` 1 回読み上げ) で告知。push:* 行が disabled→有効に変わった場合は該当行も flash + group 移動"
  - "active session 切替後、cursor が指していた tool が disabled→有効や有効→disabled に変わった / 行位置が group 移動した場合、cursor は selectedToolId ベースで再計算する。同 index に別 tool が来る silent footgun を回避"
  - "IME 変換中 (composing=true) は pointer click / Space chip toggle / Alt+W / Alt+H もすべて guard (Enter と同様に変換確定が優先)"
  - "ParamSelectPhase 内では本タスクで listbox / chip の挙動を変えるが useDynamicParamPreset の preselect / FR-A4 empty-state は維持"
  - "Active context header の projBase 表示で projects[].path の basename を使うが、(a) path が '/' 終端や empty なら path そのものを fallback、(b) 同名 basename が複数 projects に存在する場合は disambiguator として親 dir 名 (例: `work (under foo)`) を併記、(c) Windows path (\\ 区切り) でも basename が抽出できるよう / と \\ 両方を separator として扱う"
  - "sessionID prefix 8 char は uniqueness を保証しないため、tooltip で full sessionID も提供する。複数 session が prefix 8 char 衝突する場合でも toast の sessionID は monospace + tooltip full でユーザーが事後検証できる"
  - "view-update で sessionConfig.pushCommands が新規に増えた / 減ったときに listbox は次 render で反映。cursor が削除された行を指していたら selectedToolId ベースの再計算で safe な行に移動"
  - "screen reader: aria-activedescendant は cursor 位置 (movable な行) のみ参照。disabled 行は aria-disabled=true で `unavailable` と告知される。Active context 行は role='status' aria-live='polite' を持ち変化時に読み上げられる"
  - "disabled / `No active session` / 各 status は色だけでなく icon (warning icon / `—` prefix / spinner 等) でも区別され WCAG 1.4.1 (Use of Color) を満たす"
  - "ctx 構築失敗 (httpFactory invalid) 状態では Active context 行を描画せず status badge slot に `Unavailable` を表示する (palette 全体不通であることを矛盾なく示す)"
  - "chip の visibility (showWorktreeToggle / showHostToggle) が project 選択後に動的に変わるとき、focus が消える chip にある場合は focus を command 入力欄に戻す (focus trap 内 fallback ルール)"
  - "submitting 中の context shift は palette UI を凍結 (Active context 行 / listbox / status badge を変更しない)。送信解決後の次回 palette open で新 active を反映 (silent context switch + 送信先ズレを防ぐ)"
assumptions: []
related:
  adrs: [0036, 0040, 0046, 0047, 0049, 0050, 0051, 0052, 0053]
  prior_specs:
    - docs/specs/2026-06-24-web-ui-command-palette/spec.md
    - docs/specs/2026-06-25-palette-bugfix/spec.md
---

# UX — Web UI コマンドパレット 再設計 (TUI 移植からの Web 最適化, 案 A 最小)

## Goal

Web UI コマンドパレット (arc TUI prefix+p / prefix+C-p の Web 移植) は commit 9287c7f の時点で TUI 操作モデルをほぼそのまま写しており、Web UX 標準と複数箇所で乖離している。具体的には (i) ScopeSegment による standard / push の tab 分割、(ii) push tool が disabled のとき push tab 全体が空 list になり理由がユーザーに見えない、(iii) Worktree / Host chip の toggle が Tab / Shift+Tab というキーバインドで Web の Tab 標準 (focus 移動) と衝突、(iv) pointer hover と keyboard cursor で highlight が独立して動く、(v) パレット内に active session の文脈表示が無く push の送信宛先が不可視、(vi) paramless push 送信後にフィードバックが薄く誤送信検知が困難、の 6 点である。

本 UX 再設計は案 A (最小) として、これらを 1 PR で観測可能な振る舞いに落とす。すなわち (a) ScopeSegment を撤去して全 tool を 1 listbox に統合し『有効グループ → separator → disabled グループ』に並べる、(b) disabled tool も理由 (disabledReason) 付きで常時表示し keyboard cursor が skip する、(c) chip toggle を pointer click / Tab→Space / Alt+W / Alt+H + chip Enter の 4 経路に再設計、(d) hover と keyboard を単一 cursor state に統一する (有効行 hover で cursor follow、disabled 行 hover は cursor を奪わない)、(e) palette header に client-local active session (ADR-0046) を常時表示し変化時は flash + aria-live で告知、(f) paramless push の送信時 toast に projBase + sid8 + full path tooltip を載せる、を観測可能な振る舞いとして実現する。ADR-0036 (store 純粋性) / ADR-0040 (IME) / ADR-0046 (stale-tab + client-local active) / ADR-0047 (disabledReason 単一情報源) / ADR-0049 (英語統一) はすべて維持し、変更は `src/client/web/` 内に閉じる。

## Target Users

- **arc Web UI で複数セッション (new-session + push) を頻繁に開閉する個人開発者** — 一日に何度も palette を開き push の送信先を判断したい。送信宛先が不可視だと誤送信のコストが高い。
- **TUI 版 arc に慣れていて Web 版に移行しつつあるユーザー (worktree / host を頻繁に切り替える)** — TUI の Tab / Shift+Tab chip 切替を体で覚えているが、Web の Tab 標準と衝突するため retraining が必要。新キーバインド (Alt+W / Alt+H) と key hint icon でこのギャップを緩和する。
- **Web 慣れだが TUI を触らないユーザー (Tab / Shift+Tab を知らず pointer 操作主体)** — Raycast / Linear / VSCode の command palette と同じ操作感を期待する。1 list 統合と pointer click による chip toggle が必須。
- **アクセシビリティ支援技術 (NVDA / VoiceOver) で操作するキーボードユーザー** — disabled 行の理由を screen reader で読みたい。active session 変化を aria-live で受け取りたい。色のみに依存しない (WCAG 1.4.1)。

## Primary Flows

### F-001 パレット起動 → 統合 list から new-session を選び送信 (正常系)

**狙い**: 1 listbox 統合と active context header と chip 4 経路 toggle が、典型ユースケースで一貫した正常パスを構成することを確認する。
**前提**: snapshot.projects が N>=1 件存在し、ScopeSegment が描画されない統合 list 構成になっている。preselect 経路ではなく ToolSelectPhase 経由の標準入口を辿る。

### F-002 push tool が disabled な状態で list 上に表示・選択ブロック

**狙い**: disabled tool を visible に残し、選択操作 (Enter / pointer click) は palette を閉じずに inline status + 行 flash で告知する設計を、active session 未選択という典型 disabled 条件で確認する。
**前提**: Web UI を開いた直後で active session が未選択。push:* が全件 disabled な状態。

### F-003 hover follow と keyboard cursor の単一 highlight

**狙い**: pointer hover と keyboard cursor が同じ cursor state を共有する単一 highlight モデルを、有効行 / disabled 行 / listbox 外退出の 3 ケースで観測する。
**前提**: 有効行が複数並び keyboard で navigate 可能な状態。

### F-004 worktree / host chip を pointer + Alt+W / Alt+H + Tab→Space の 3 経路で toggle

**狙い**: Tab / Shift+Tab chip 切替を撤去し pointer click / Alt hotkey / Tab→Space + chip Enter toggle の 4 経路を確保する。Web の Tab 標準 (focus 移動) が復元されていることも観測する。
**前提**: ParamSelectPhase の command 入力欄にフォーカス、Worktree / Host 両 chip が visible (project が isGit かつ isSandboxed)。

### F-005 paramless push の送信先明示 toast

**狙い**: paramless push 送信時に projBase / sid8 / full path tooltip を toast で明示し、誤送信を事後に検知可能にする。
**前提**: active session が存在し pushCommands が available な状態。

### F-006 active session の palette open 中切替を flash + aria-live で告知

**狙い**: palette open 中に別 client から active session 切替の view-update が届いたとき、silent な context shift を構造的に防ぐ (header flash + aria-live + 該当行 group 移動 + 行 flash)。
**前提**: 複数 client が同じ daemon に接続しており、別 client 側で active session が切り替わる状況。

### F-007 disabled tool は cursor が skip し、有効グループに着地する

**狙い**: ↑↓ navigation の skip ルールと、有効行 0 件の異常状態を status badge slot で明示する silent-failure 禁止ルールを観測する。
**前提**: listbox に有効 / disabled が混在しているか、sessionConfig 未 hydrate で operable 0 件。

### F-008 submit in-flight 中に active 切替が起きた場合の凍結

**狙い**: 送信中に active が変わっても palette UI を凍結し、表示と送信先のズレを構造的に防ぐ。
**前提**: push:save の Enter を押し送信中 (status badge slot `Sending…` spinner 表示中)。

## Acceptance Scenarios

### F-001

**UAC-001**
- Given: ユーザーが Web UI を開き TerminalPane にフォーカスがある状態
- When: prefix+p を押す
- Then: CommandPalette overlay が画面に表示され、入力欄にカーソルが点滅し、最上部に Active context 行が見える

**UAC-002**
- Given: active session が未選択 (画面右上に session 表示がない状態) で palette を開いた直後
- When: header 行を視認する
- Then: `— No active session` というテキストが icon 付きで表示され、color のみに依存せず無効状態が認識できる

**UAC-003**
- Given: palette が開いていて listbox の先頭 (New Session) に cursor がある状態
- When: Enter を押し Project の先頭オプションを Enter で選び command 入力欄に `hello` と入力して Enter を押す
- Then: palette overlay が画面から消え、新しいセッションが画面に追加表示され、palette 起動前にフォーカスしていた要素にフォーカスが戻る

### F-002

**UAC-004**
- Given: active session が未選択の状態で palette を開いた直後
- When: listbox を視認する
- Then: [New Session] が上、separator (横線) を挟んで [push:save (warning icon + `No active session`), push:resume (同様)] が下に表示されている

**UAC-005**
- Given: 上記状態で push:save 行に cursor がある (もしくは pointer で hover している)
- When: Enter を押す
- Then: palette は開いたままで、該当行が 1 回 flash し、入力欄直下に `"save" is unavailable: No active session` のテキストが表示され screen reader が同じ文言を読み上げる (toast 通知は出ない)

### F-003

**UAC-006**
- Given: palette listbox に有効な行が 7 件並んでいて、keyboard cursor が 4 行目 (aria-selected=true) にある状態
- When: ユーザーが pointer で 6 行目 (有効) に hover する
- Then: 6 行目に aria-selected=true の highlight が移動し、4 行目からは highlight が消える

**UAC-007**
- Given: keyboard cursor が有効な行に置かれた状態で、ユーザーが listbox 内の disabled 行に pointer を hover した状態
- When: Enter を押す
- Then: 発火するのは元の keyboard cursor 行 (有効な行) であり、disabled 行は発火しない (palette は閉じ、もしくは次 phase に進む)

### F-004

**UAC-008**
- Given: command 入力欄にフォーカスがあり Worktree chip (aria-checked=false) が見える状態
- When: ユーザーが Worktree chip を pointer click する
- Then: Worktree chip の aria-checked が true になり、command 入力欄のキャレットが消えずフォーカスが維持されている

**UAC-009**
- Given: command 入力欄にフォーカスがあり Worktree chip (aria-checked=false) が見える状態
- When: Alt+W を押す
- Then: Worktree chip の aria-checked が true になる (pointer click と同じ視覚結果)

**UAC-010**
- Given: Worktree chip にフォーカスがある (chip が focus ring を持つ) 状態
- When: Enter を押す
- Then: chip の aria-checked が toggle され、palette overlay は閉じない (form submit が発生しない)

### F-005

**UAC-011**
- Given: Active context 行に `Active: bar / sess_abcd1234` と表示されている palette open 状態
- When: `save` を入力して Enter を押す
- Then: palette が閉じ、画面右下に info 色の toast `Sent 'save' → bar · sess_abcd1234` が表示され、sessionID 部分が monospace で読める

**UAC-012**
- Given: 上記 toast が表示されている状態
- When: toast 上に pointer を hover する
- Then: tooltip でフルプロジェクトパス (例: `/home/dev/foo/bar`) とフルセッション ID が表示される

### F-006

**UAC-013**
- Given: palette が開いていて Active context 行に `Active: foo / sess_001aaaa` が表示されている状態
- When: 別 client の操作によりこの client の active session が sess_002bbbb に切り替わる
- Then: Active context 行が `Active: bar / sess_002bbbb` に変わり、視覚的に約 600ms flash し、screen reader が `Active session changed to bar / sess_002bbbb` を 1 回読み上げる

**UAC-014**
- Given: 上記の active 切替により push:save 行が disabled → 有効に変化した状態
- When: listbox を視認する
- Then: push:save 行から warning icon と `No active session` 文言が消え、separator の上段 (有効グループ) に移動して 1 回 flash している

### F-007

**UAC-015**
- Given: listbox に [New Session, separator, push:save (disabled), push:resume (disabled), push:status (有効)] が並び keyboard cursor が New Session にある状態
- When: ↓ を 1 回押す
- Then: keyboard cursor highlight が push:status に表示され、push:save と push:resume には cursor highlight が出ない (ただし両行は list 上に表示されたままで warning icon + `No active session` が読める)

**UAC-016**
- Given: sessionConfig がまだ hydrate されておらず list に operable な行が 1 件も無い状態
- When: palette を視認する
- Then: palette 上部の status badge slot に `Loading commands…` テキストが表示され、Enter を押しても何も起きないことがメッセージで明示される (silent ではない)

### F-008

**UAC-017**
- Given: push:save の Enter を押し palette 上部 status badge slot に `Sending…` spinner が見える状態
- When: 送信解決前に view-update で active session 切替が届く
- Then: palette の Active context 行 / listbox 表示は変化せず、`Sending…` 表示が継続している

**UAC-018**
- Given: 上記の送信が解決した瞬間
- When: 送信完了 response が返る
- Then: palette が閉じ、F-005 と同じフォーマットの info toast が表示される (送信先は送信開始時の active context のまま)

## Edge Cases

- **disabled 行 0 件の silent failure 禁止**: ↑↓ で skip 対象が無いとき no-op だが、status badge slot に `Loading commands…` / `No commands available` を明示してユーザーに状況を伝える。
- **hover と cursor の単一 state**: pointer と keyboard は同じ cursor state を共有。disabled 行 hover では cursor state を更新せず subtle hover style のみ。Enter は常に keyboard cursor 行で発火する。
- **Stop session lifecycle action の再導入禁止**: e4fd31d で撤去された経緯 + 案 A 最小スコープ + 不可逆操作の確認モーダル要求から、本タスクでは復活させない。
- **Tab / Shift+Tab chip 切替の撤去**: Tab は素の focus trap 内移動として復元。chip toggle は pointer click / Tab→Space / Alt+W / Alt+H の 4 経路。chip focus 中 Enter は toggle (submit にならない)。
- **global header (palette 外) の active session 表示は scope 外**: palette 内 header のみが本タスクの責務。
- **list 並び順は『有効 → separator → disabled』固定**: グループ内は registry 順を維持。recently used 等の dynamic sort は scope 外。
- **disabled inline 文言の single source**: `"<label>" is unavailable: <reason>` の `<reason>` は scopeDisabledReason の生戻り値を加工せず埋め込む (ADR-0047)。
- **paramless push toast の責務**: 送信先 projBase + sid8 を明示し tooltip で full path / full sessionID を提供。session 切替リンクは入れない (read-only)。
- **Active context 行の source**: client-local activeSessionID (ADR-0046) のみ。daemon-global active は参照しない。submit in-flight 中は palette 全体を凍結。
- **Active context 変化の告知**: 約 600ms background flash + aria-live=polite で `Active session changed to ...` を 1 回読み上げ。disabled → 有効に変わった push 行も flash + group 移動。
- **cursor 再計算は selectedToolId ベース**: active 切替で行位置が動いても同 index に別 tool が来る silent footgun を回避するため、selectedToolId で再計算する。
- **IME composing 中の guard 拡張**: pointer click / Space chip toggle / Alt+W / Alt+H もすべて Enter と同様に guard (ADR-0040)。
- **既存 FR-A4 / useDynamicParamPreset の維持**: ParamSelectPhase 内 listbox / chip の挙動は本タスクで変えるが preselect と empty-state は維持。
- **projBase basename 抽出ルール**: (a) path が `/` 終端 / empty なら path 全体 fallback、(b) 同名 basename 複数なら parent dir 名で disambiguate、(c) `/` と `\` 両 separator を扱う。
- **sessionID prefix 8 char の uniqueness 不保証**: tooltip で full sessionID を必ず提供。衝突時もユーザーが事後検証できる。
- **sessionConfig.pushCommands の動的増減**: 次 render で listbox に反映。cursor が削除行を指していたら selectedToolId ベース再計算で safe 着地。
- **screen reader 配線**: aria-activedescendant は movable な行のみ参照。disabled 行は aria-disabled=true で `unavailable` 告知。Active context 行は role='status' aria-live='polite'。
- **WCAG 1.4.1 (Use of Color)**: disabled / No active session / loading / submitting / unavailable はすべて icon + prefix 文字を併用し色非依存。
- **ctx 構築失敗 (httpFactory invalid)**: Active context 行は描画せず status badge slot に `Unavailable`。palette 全体不通であることを矛盾なく示す。
- **chip visibility 動的変化での focus 喪失**: focus が消える chip にあれば command 入力欄に戻す (focus trap 内 fallback)。
- **submit 中 context shift の凍結**: Active context 行 / listbox / status badge を送信解決まで変更しない。次回 palette open で新 active を反映。

## Open Questions

> 未解決 UX 判断。確定したら ADR に格上げ or frontmatter 反映する。

- **[本タスク out-of-scope]** global header (palette 外) の active session 常時表示を追加するか — 別 issue で UX 検討。
- **[将来 ADR 候補]** 1 list 内の並び順に recently used / 使用頻度に基づく動的 sort を導入するか — 案 C 領域。pushCommands が中期的に増えた場合の備え。
- **[将来 ADR 候補]** push tool entry に destructive フラグを追加し destructive な push (例: reset / quit) には確認モーダルを表示する分類設計を導入するか — active 切替起因の 1 keystroke 誤送信を構造的に防ぐ将来 ADR 候補。
- **[本タスク out-of-scope]** Stop session lifecycle action を palette に復活させるか — e4fd31d で撤去された経緯あり。復活させるなら確認モーダル + ADR 必須。
- **[実機検証]** Alt+W / Alt+H の hotkey が他 OS / ブラウザの mnemonic とぶつかる場合のフォールバック — 例: macOS Option+W の特殊文字入力との競合の有無を実機検証する必要。
- **[将来 ADR 候補]** pushCommands が中期的に何件まで運用上想定されるか assumption に明記すべきか — 現状無制限。100 件超で listbox UX が破綻するライン特定。
