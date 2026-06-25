# ADR 0053 — Worktree / Host chip の Tab・Shift+Tab toggle を撤去し pointer + Alt+W・Alt+H + Tab→Space + chip Enter の 4 経路に再設計

Status: Accepted

Related: [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: F-004 (UAC-008〜UAC-010)

## Context

commit 9287c7f の ParamSelectPhase は new-session の Worktree / Host chip を **Tab で Worktree toggle / Shift+Tab で Host toggle** というキーバインドで操作する。これは arc TUI 版の chip 操作モデルを直訳したもので、TUI では Tab は単に key event の一つだが、Web の Tab は『focus を次の interactive 要素に動かす』というブラウザ標準動作を持つ。Tab を chip toggle に奪うと:

- focus trap 内で「次の chip にフォーカスを動かす」「palette 外に focus を出す」といった素直な Tab 操作が永久に塞がれる
- chip 自体が role='switch' を持たず pointer click / Space / Enter どれでも toggle できないため、**pointer ユーザーは chip を全く操作できない** (Web では完全な締め出し)
- screen reader ユーザーが chip にフォーカスを当てる手段が無い (Tab で来られないため)

つまり TUI 直訳のキーバインドが Web UX の 3 大入力経路 (pointer / keyboard tab / screen reader) を同時に塞いでいる。これを最小スコープで解消する設計判断を本 ADR で決める。spec.md / plan.md は後続 plan-how フェーズで生成される。

## Decision

(1) chip を **role='switch'** にし `aria-checked` (true / false) を持たせる。これにより pointer click / keyboard Space で toggle が発火する Web ARIA 標準に乗せる。pointer click 時は `event.preventDefault()` + `event.stopPropagation()` で input 欄 focus を維持する (フォーカスは command 入力欄に残す)。

(2) Tab / Shift+Tab の chip toggle 専用キーバインドを**撤去**する。Tab は素の focus trap 内移動 (`focus-trap-react` 互換) として復元され、command 入力欄 → Worktree chip → Host chip → command 入力欄 のサイクルで focus が動く。Shift+Tab は逆順。

(3) Tab で chip に focus を当てた後の **Space** で chip toggle (ARIA 標準)。chip focus 中の **Enter** も toggle (form submit にはならない。submit は input 欄 focus 時の Enter に限定)。chip focus 中 Enter の挙動を明示するのは、submit と toggle が混同される footgun を防ぐため。

(4) palette open 中限定で **Alt+W = Worktree toggle / Alt+H = Host toggle** の global hotkey を追加する。input 欄 focus 中でも chip focus 中でも発火する。chip 左に `[W]` / `[H]` の key hint icon を常時表示し、affordance を視覚的に提供する (新 hotkey の存在をユーザーが知るための装置)。

(5) chip visibility (`showWorktreeToggle` / `showHostToggle`) が project 選択後に動的に変わるとき (例: 非 git project に切り替えると Worktree chip が消える)、focus が消える chip にある場合は **focus を command 入力欄に戻す** (focus trap 内 fallback ルール)。focus が失われて document.body に逃げる挙動を構造的に防ぐ。

(6) IME composing 中 (`composing=true`) は pointer click / Space / Alt+W / Alt+H / chip Enter のすべてを guard する (ADR-0040)。Enter と同じ扱いで、変換確定が優先される。

## Consequences

- **positive**: pointer ユーザーが chip を click で操作可能になり、Web UX の完全な操作経路が確保される (現状の締め出しが解消)。
- **positive**: Web の Tab 標準 (focus 移動) が復元され、focus trap 内で素直にナビゲートできる。screen reader ユーザーも chip にアクセス可能。
- **positive**: Alt+W / Alt+H + `[W]` / `[H]` key hint icon の組合せで、新 hotkey の存在が affordance として常時 visible。隠れたショートカットにならない。
- **positive**: chip focus 中 Enter = toggle / input 欄 focus 中 Enter = submit の役割分離が明示され、submit 誤発火が構造的に防止される。
- **positive**: chip visibility 動的変化での focus 喪失も focus trap 内 fallback ルールで構造的に防止され、focus が body に逃げる footgun が消える。
- **negative**: TUI の Tab/Shift+Tab に慣れたユーザーの retraining が必要。release note と `[W]` / `[H]` key hint icon で緩和するが、筋肉記憶への影響は避けられない。
- negative: Alt+W / Alt+H が他 OS / ブラウザの mnemonic (例: macOS Option+W で `∑` 入力 / 一部ブラウザの menu mnemonic) と稀に衝突する可能性がある。実機検証は open_questions に送り、本 ADR では `event.preventDefault()` で当該 hotkey をブラウザに伝播させない実装にする。
- neutral: ARIA role='switch' + aria-checked は WAI-ARIA 1.2 標準で screen reader 対応が安定している。NVDA / VoiceOver / JAWS で同様に告知される。
- neutral: chip Enter = toggle の挙動は form 内 chip の Web 標準 (button / switch) と整合し、driver-typed input field の Enter = submit とも矛盾しない (focus 元で挙動が分岐するため)。

## Alternatives Considered

### Tab / Shift+Tab を残したまま pointer click を追加する

pointer 操作経路は確保できるが、Tab の標準動作 (focus 移動) が永久に塞がれたまま。Web の focus trap UX 標準から逸脱したままで、screen reader ユーザーが chip にアクセスする手段も無い。本 ADR の根本問題 (Tab 奪取) を解決していない。

### Alt+W / Alt+H ではなく Ctrl+W / Ctrl+H

Ctrl+W は**ブラウザのタブ閉じ**と衝突 (deal-breaker)。Ctrl+H はブラウザの履歴ページと衝突 (chrome / Firefox)。Alt は Web app の慣習的な mnemonic key で衝突が少ない。

### chip Enter で form submit する

form 内 chip の Enter は ARIA 標準で toggle が直感的 (button / switch 共に)。submit にすると chip にフォーカスがある状態の Enter で意図せず送信が発火し、特に paramless push の誤送信を誘発する。submit は input 欄 focus 時の Enter に限定するのが安全。

### chip を `<input type="checkbox">` で実装する

ARIA `role='switch'` と同等の意味論だが、checkbox は通常 label + box の 2 要素レイアウトで chip のデザインに合わない。role='switch' の単一 button element で chip 形状を保ちつつ ARIA 意味論を満たす実装が綺麗。

### Alt+W / Alt+H の hotkey を導入せず pointer + Tab→Space のみで済ます

key hint icon `[W]` / `[H]` の affordance が失われ、keyboard 操作で 1 手数 (Tab で chip に focus → Space で toggle) 必要になる。input 欄から手を離さず toggle できる Alt hotkey が utility として顕著に高く、TUI ユーザーの retraining も Alt+W / Alt+H という新ルールで明示できる。
