---
id: ux-20260626-web-terminal-mobile-ux
kind: ux
title: "Web UI ターミナルタブ (TerminalPane) のモバイル UX — 閲覧/入力モード分離"
status: draft
created: 2026-06-26
owners: [take.gn@gmail.com]
goal: "Web UI ターミナルタブ (TerminalPane) のモバイル UX を「閲覧モード (デフォルト) / 入力モード」に明示分離し、(a) タップで仮想キーボードが勝手に出ない、(b) 指スワイプで scrollback がスクロールしつつ long-press で文字選択を維持、(c) キーボード FAB で明示的に入力モードへ、(d) scrollback 中だけ ↓最新 FAB、(e) 2 本指ピンチ + 支援技術向けステッパで fontSize 変更 + refit、を観測可能な振る舞いとして実現する。PC (keyboard / mouse wheel / 選択コピー) は完全に現状維持。gate は画面幅 AND pointer:coarse で判定し、FAB は terminal-host の flex layout (ADR-0029) に干渉しない overlay 配置とする。"
target_users:
  - "スマホから既存セッションに late-join して状況確認 (scrollback で過去出力を遡る) したい運用者"
  - "スマホ/タブレットから確認応答・short input (y/n, 1 行コマンド) だけ軽く打ちたい運用者"
  - "PC で keyboard 入力・mouse wheel scroll・ドラッグ選択コピーを使う既存ユーザー (現状維持対象)"
  - "VoiceOver/TalkBack を使うモバイル支援技術ユーザー (FAB は 44px + aria-label 必須、pinch 不能でも font サイズ変更に到達できること)"
primary_flows:
  - id: F-001
    name: "モバイル mode 分離 — 閲覧モード既定と入力モードへの明示遷移/退出"
    steps:
      - "[pointer-touch|screen-reader] モバイル gate (matchMedia (max-width: 767px) and (pointer: coarse) が true) でセッションを開くと、terminal-host ラッパは data-input-active=\"false\" (閲覧モード) で起動し、helper textarea は readonly 属性を持ち focus されていない"
      - "[pointer-touch] 閲覧モードで xterm 表示領域を tap しても helper textarea へ focus が移らず (activeElement は変わらない)、仮想キーボードは出ない (focus 移動のブロックが keyboard 抑止の load-bearing 機構、readonly は defense-in-depth)"
      - "[pointer-touch] 初回の閲覧モード突入時のみ、キーボード FAB 隣に dismissible な coachmark (タップで入力 / 2 本指で文字サイズ) を出す。tap または数秒で消え、以後は localStorage キー web.term.hintSeen で再表示しない"
      - "[pointer-touch|screen-reader] キーボード FAB (キーボード glyph, aria-label=\"キーボードを開く\", aria-pressed=\"false\") を tap すると data-input-active=\"true\" に遷移し、helper textarea の readonly が外れ focus され仮想キーボードが立ち上がる。FAB は pointerdown を抑止して focus を奪わないため、enter と同時の blur-exit は起きない"
      - "[keyboard] 入力モードで文字を打つと従来経路 (term.onData → conn.send k:i) でセッションへ送信される (入力経路は legacy 踏襲)"
      - "[pointer-touch|screen-reader] 入力モードで同じ FAB (aria-label=\"キーボードを閉じる\", aria-pressed=\"true\") を再 tap すると data-input-active=\"false\" に戻り helper textarea が blur + readonly 再付与"
      - "[pointer-touch] 入力モードで terminal 表示領域 (helper textarea ではなくコンテンツ部) を tap (領域外タップ相当) すると data-input-active=\"false\" に戻る"
      - "[pointer-touch|keyboard] OS のキーボード非表示 (textarea blur) または hardware keyboard の Esc で data-input-active=\"false\" へ同期し、aria-live=polite 領域が「閲覧モードに戻りました」を 1 回読み上げる"
      - "[pointer-touch] 入力モード中、仮想キーボードが visualViewport を縮めても キーボード FAB は visualViewport 上端より上に保たれ、再 tap での退出が常に到達可能"
  - id: F-002
    name: "閲覧モードの touch swipe による scrollback スクロール"
    steps:
      - "[pointer-touch] 閲覧モードで .xterm-viewport は touch-action:pan-y を持ち、縦方向の指スワイプを browser の scroll に割り当てる"
      - "[pointer-touch] 上方向スワイプで .xterm-viewport.scrollTop が減り過去行 (scrollback) が見える"
      - "[pointer-touch] 下方向スワイプで scrollTop が増え最新側へ戻る"
      - "[pointer-touch|screen-reader] スクロールしても入力モードには遷移せず helper textarea は focus されない (swipe が入力に吸われない)"
  - id: F-003
    name: "閲覧モードの文字選択維持 — long-press 起点の選択は swipe/scroll と判別"
    steps:
      - "[pointer-touch] 閲覧モードで terminal コンテンツ上を long-press (約 500ms 静止保持) すると選択モードに入り、以後のドラッグは scroll ではなく文字選択になる (.xterm-selection-layer に選択範囲が描画)"
      - "[pointer-touch] 選択後はネイティブの選択ハンドル/コピー操作が使え、term.getSelection() が非空になる"
      - "[pointer-touch|screen-reader] 選択操作中も入力モードに遷移せず helper textarea は focus されない / 仮想キーボードは出ない"
      - "[pointer-touch] long-press を伴わない素早い縦 swipe は F-002 どおり scroll になり選択しない (long-press dwell で判別)"
  - id: F-004
    name: "↓最新 FAB — scrollback 中 (mode 非依存) のみ表示し末尾へ復帰"
    steps:
      - "[pointer-touch] scrollTop が末尾 (scrollHeight-clientHeight に ±2px 近接を末尾扱い) のとき ↓最新 FAB は DOM 不在/hidden"
      - "[pointer-touch] 閲覧/入力どちらでも scrollback 中 (scrollTop≠末尾) になると ↓最新 FAB (aria-label=\"最新へスクロール\") が overlay 表示される"
      - "[pointer-touch|screen-reader] FAB を tap すると .xterm-viewport が末尾までスクロールする。prefers-reduced-motion:reduce 時は即時ジャンプ、それ以外は smooth (ADR-0064 の単一 reduced-motion guard ブロックに追記)"
      - "[pointer-touch] 末尾到達後 FAB は自動的に非表示へ戻る"
      - "[screen-reader] FAB 出現時、aria-live=polite が「最新へ移動できます」を 1 回読み上げる"
  - id: F-005
    name: "fontSize 変更 — 2 本指ピンチ + 支援技術向けステッパ (clamp + 永続化)"
    steps:
      - "[pointer-touch] .xterm-viewport で 2 本指 pinch を開始すると 2 指間距離の比率で目標 fontSize を連続算出する (touches.length=2 で pinch、1 で swipe を判別)"
      - "[pointer-touch] pinch out で fontSize が増え pinch in で減る。範囲は min 8px / max 28px / default 14px に clamp する"
      - "[pointer-touch] pinch 中、画面中央に現在 fontSize の transient indicator (例 16px) を表示し touchend 約 800ms 後に fade させる (既存 Toast primitive を再利用)。indicator の tap で default 14px へ reset する"
      - "[pointer-touch] fontSize 変更ごとに ADR-0034 の scheduleFit (rAF coalesce) 経由で fit() を呼び cols/rows を再計算し refit する"
      - "[pointer-touch|screen-reader] pinch 不能/支援技術ユーザー向けに、font-size 制御ボタン (aria-label=\"文字サイズ\") から ＋ / － / リセット(14px) を選べる非 pinch の代替経路を提供する (各 44px, role=button)"
      - "[pointer-touch] pinch 終了時の確定 fontSize を device-scoped localStorage キー web.term.fontSize に保存し、次回起動時に [8,28] へ clamp 検証してから復元する (不正値/範囲外は default 14px)"
  - id: F-006
    name: "PC 非影響 — pointer:fine 環境で legacy 挙動を完全維持 (narrow-width AND coarse 判別)"
    steps:
      - "[keyboard|pointer-mouse] gate false (例 1280px / pointer:fine) のとき、モバイル overlay (キーボード FAB / ↓最新 FAB / font-size 制御) は一切 render されない"
      - "[keyboard|pointer-mouse] narrow 幅でも pointer:fine (例 700px の desktop window + マウス) なら gate false = desktop 扱い (幅 AND coarse の AND 契約)"
      - "[pointer-mouse] terminal 表示領域を click すると従来どおり helper textarea が focus し入力できる (readonly 無し)"
      - "[pointer-mouse] mouse wheel で .xterm-viewport が scrollback をスクロールし、ドラッグで選択コピーできる (legacy 踏襲)"
      - "[pointer-mouse] .xterm-viewport の touch-action 等モバイル指定は適用されず、xterm の既定挙動を維持する"
  - id: F-007
    name: "アクセシビリティ — FAB の 44px タッチターゲット / aria 名・状態 / overlay box 不変 / token 整合"
    steps:
      - "[pointer-touch|screen-reader] キーボード FAB / ↓最新 FAB / font-size 制御は <button> で getBoundingClientRect が 44×44px 以上 (FR-A11Y-001) かつ非空の aria-label を持つ"
      - "[screen-reader] キーボード FAB は aria-pressed でトグル状態を公開し、state 変化に同期する"
      - "[pointer-touch] FAB は terminal-slot の absolute overlay 子として配置し terminal-host (flex:1 1 0, ADR-0029) の box サイズに影響しない。safe-area は .app-shell が四辺適用済み (FR-LAYOUT-004) のため FAB 側で env(safe-area-inset) を再加算しない (親 inset 内に 16px offset で配置)"
      - "[pointer-touch] 複数 FAB は固定スタック順 (下: キーボード FAB → 上: ↓最新 FAB を 8px gap) で重ならず、既存 .notification-toast と z-index/位置を分離する"
      - "[pointer-touch|pointer-mouse|screen-reader] FAB の色は既存トークン (--accent / --surface-*) と ADR-0059 theme 連動を用い、light/dark/コントラストで破綻しない"
acceptance_scenarios:
  - id: UAC-001
    flow_ref: F-001
    given: "モバイル gate (max-width: 767px and pointer: coarse) が true の環境でセッションを開き terminal-slot[data-active=true] が可視"
    when: "セッション初期描画が完了する"
    then: "terminal-host ラッパに data-input-active=\"false\" 属性があり、document.activeElement は .xterm-helper-textarea ではなく、.xterm-helper-textarea に readonly 属性が存在する"
    vs_legacy: must-fail
    counterexample: "誤実装: モバイル判定を入れず legacy のまま render する。data-input-active 属性自体が DOM に存在せず attribute assertion が undefined で fail し、legacy は readonly を付与しないため readonly assertion でも fail する。"
  - id: UAC-002
    flow_ref: F-001
    given: "モバイル gate true・閲覧モード (data-input-active=\"false\")・helper textarea に focus イベントリスナを装着済み"
    when: "xterm 表示領域の中央座標を tap (touchstart→touchend) する"
    then: "tap 完了後 document.activeElement は依然 .xterm-helper-textarea でなく、tap 中に textarea へ focus イベントが 1 度も dispatch されない"
    vs_legacy: must-fail
    counterexample: "誤実装: touchend で term.focus() した直後に setTimeout(()=>term.blur()) で blur する チラ見せ 方式。focus イベントが実際に dispatch されるため focus イベント発火数 0 の assertion で fail する。(初稿の visualViewport.height 不変 assertion は Playwright で実キーボードが立たず常に真=判別力ゼロなので採用せず、focus 発火数のみを判別軸にする)"
  - id: UAC-003
    flow_ref: F-001
    given: "モバイル gate true・閲覧モード・キーボード FAB が aria-pressed=\"false\" で可視"
    when: "キーボード FAB を tap し、tap 直後 200ms 経過まで観測する"
    then: "terminal-host ラッパが data-input-active=\"true\" になり、document.activeElement === .xterm-helper-textarea かつ helper textarea に readonly 属性が無く、FAB の aria-pressed=\"true\"・aria-label=\"キーボードを閉じる\" に変わり、200ms 経過後も data-input-active が \"true\" のまま (enter→即 exit の flicker が無い)"
    vs_legacy: irrelevant
    counterexample: "誤実装A: FAB tap で focus はするが readonly を外し忘れる → iOS Safari は readonly textarea で仮想キーボードを出さず入力できないため readonly 不在 assertion で fail。誤実装B: FAB が pointerdown で focus を奪い blur-exit listener が発火 → enter 直後に data-input-active が false へ戻り 200ms 後の \"true\" 維持 assertion で fail (enter/exit race)。"
  - id: UAC-004
    flow_ref: F-001
    given: "モバイル gate true・入力モード (data-input-active=\"true\", aria-pressed=\"true\")"
    when: "同じキーボード FAB を再 tap する"
    then: "data-input-active=\"false\" に戻り、document.activeElement が .xterm-helper-textarea でなくなり、helper textarea に readonly が再付与され、FAB aria-pressed=\"false\""
    vs_legacy: irrelevant
    counterexample: "誤実装: FAB を単発トリガーにして toggle にしない (常に focus を試みる)。再 tap 後も data-input-active が \"true\" のままで aria-pressed=\"true\" なので false への遷移 assertion で fail する。"
  - id: UAC-005
    flow_ref: F-001
    given: "モバイル gate true・入力モード・helper textarea が focus 済み"
    when: "terminal の表示コンテンツ部 (helper textarea 以外) を tap する (領域外タップ相当)"
    then: "data-input-active=\"false\" になり FAB aria-pressed=\"false\" に同期する"
    vs_legacy: irrelevant
    counterexample: "誤実装: outside-tap を購読せず、xterm がコンテンツ tap で helper textarea を再 focus するだけにする。data-input-active=\"true\" のまま残り false assertion で fail する (要件「領域外タップで閉じる」未達)。"
  - id: UAC-006
    flow_ref: F-001
    given: "モバイル gate true・入力モード・helper textarea が focus 済み"
    when: "helper textarea を programmatic に blur する (OS のキーボード非表示=blur 相当) もしくは Esc keydown を送る"
    then: "data-input-active=\"false\" になり FAB aria-pressed=\"false\" に同期し、aria-live=polite 領域に「閲覧モードに戻りました」のテキストが現れる"
    vs_legacy: irrelevant
    counterexample: "誤実装: 入力モード state を FAB クリックハンドラだけで管理し blur/Esc を購読しない。OS がキーボードを閉じた後も data-input-active=\"true\" の幽霊状態が残り、blur 後の false assertion と aria-live assertion で fail する。"
  - id: UAC-007
    flow_ref: F-002
    given: "モバイル gate true・閲覧モード・scrollback に画面 2 枚分以上の行があり .xterm-viewport.scrollTop が最大値 (末尾)"
    when: ".xterm-viewport 上で下から上へ 200px の touch swipe (touchstart→touchmove×N→touchend) を行う"
    then: ".xterm-viewport.scrollTop がスワイプ前より小さくなり (過去行が表示)、document.activeElement は .xterm-helper-textarea でなく data-input-active=\"false\" のまま"
    vs_legacy: must-fail
    counterexample: "誤実装: touch-action を付けず legacy のまま。.xterm-viewport は touch-action:auto を継承し xterm が touchmove を握る/あるいは body スクロールに化けて scrollTop が変化しないため、scrollTop 減少 assertion で fail する。"
  - id: UAC-008
    flow_ref: F-002
    given: "モバイル gate true・閲覧モード・scrollTop を最大値の半分に手動設定済み"
    when: "上から下へ 200px の touch swipe を行う"
    then: ".xterm-viewport.scrollTop がスワイプ前より大きくなり末尾方向へ戻り、data-input-active=\"false\" のまま"
    vs_legacy: must-fail
    counterexample: "誤実装: touch swipe を JS で握って 1 swipe=入力モード移行 にしてしまう。scrollTop が動かず data-input-active が \"true\" になるため、scrollTop 増加 assertion と false 維持 assertion の両方で fail する。"
  - id: UAC-009
    flow_ref: F-002
    given: "モバイル gate true・閲覧モード・helper textarea に focus リスナ装着"
    when: "scrollback 領域で縦 swipe を 3 回連続で行う"
    then: "swipe 中・後を通じ helper textarea へ focus イベントが 0 回、data-input-active=\"false\" を維持する"
    vs_legacy: must-fail
    counterexample: "誤実装: swipe の touchstart を tap と区別できず touchstart で focus する。swipe 開始時に focus イベントが発火し data-input-active が \"true\" 化するので focus 発火数 0 assertion で fail する。"
  - id: UAC-010
    flow_ref: F-003
    given: "モバイル gate true・閲覧モード・terminal に選択可能な文字列が描画されている"
    when: "terminal コンテンツ上で long-press (約 500ms 静止) してからドラッグする"
    then: "term.getSelection() が非空になり .xterm-selection-layer に選択矩形が可視で、かつ document.activeElement は .xterm-helper-textarea でなく data-input-active=\"false\" のまま"
    vs_legacy: must-fail
    counterexample: "誤実装: long-press を tap 同様に扱って term.focus() を呼ぶ。選択範囲が生成されず textarea が focus されるため、選択非空 assertion と activeElement assertion で fail する。legacy は mode 概念が無く tap で textarea を focus する (= キーボードを出す) ため必ず fail する。"
  - id: UAC-011
    flow_ref: F-003
    given: "モバイル gate true・閲覧モード・scrollTop が最大値 (末尾)"
    when: "long-press を伴わない 200px の縦 swipe を行う"
    then: ".xterm-viewport.scrollTop がスワイプ前より小さくなり、term.getSelection() は空 (選択は発生しない)"
    vs_legacy: must-fail
    counterexample: "誤実装: touchstart 即 selection 開始 (long-press dwell を無視) する。swipe で選択が走り scrollTop が動かないため、scrollTop 減少 assertion と選択空 assertion の両方で fail する。legacy は touch-action 未指定で swipe scroll 自体が成立せず scrollTop 不変なので fail する。"
  - id: UAC-012
    flow_ref: F-004
    given: "モバイル gate true・scrollTop が末尾 (scrollHeight-clientHeight と ±2px 内)"
    when: "初期描画後に DOM を観測する"
    then: "aria-label=\"最新へスクロール\" の要素が DOM に存在しない (または親が hidden 属性を持つ)"
    vs_legacy: irrelevant
    counterexample: "誤実装: ↓最新 FAB を常時 render し CSS opacity:0 で隠す。要素は accessibility tree に残り querySelector で取得できるため 存在しない/hidden assertion で fail する (screen reader にも幽霊ボタンとして読まれる)。"
  - id: UAC-013
    flow_ref: F-004
    given: "モバイル gate true・末尾に居て ↓最新 FAB 非表示"
    when: "上方向 swipe で scrollTop を末尾から 300px 以上戻す"
    then: "aria-label=\"最新へスクロール\" の button 要素が可視になり getBoundingClientRect().width≥44 かつ height≥44"
    vs_legacy: must-fail
    counterexample: "誤実装: scroll イベントを監視せず 入力モードのとき表示 など別条件で出す。末尾離脱で出現しないため可視 assertion で fail する。legacy には ↓最新 FAB 自体が存在しないため必ず fail する。"
  - id: UAC-014
    flow_ref: F-004
    given: "モバイル gate true・scrollTop が末尾から離れ ↓最新 FAB 可視"
    when: "↓最新 FAB を tap する"
    then: ".xterm-viewport.scrollTop が scrollHeight-clientHeight (末尾, ±2px) と一致し、その後 aria-label=\"最新へスクロール\" の要素が再び DOM から消える/hidden になる"
    vs_legacy: irrelevant
    counterexample: "誤実装: tap で term.scrollToBottom() を呼ぶが FAB 自身の表示条件を更新しない。scrollTop は末尾になるが FAB が出たままなので 末尾後に消える assertion で fail する。"
  - id: UAC-015
    flow_ref: F-004
    given: "モバイル gate true・入力モード (data-input-active=\"true\")・scrollback 中 (scrollTop≠末尾)"
    when: "DOM を観測する"
    then: "aria-label=\"最新へスクロール\" の button が可視である (FAB 表示は閲覧/入力の mode に依存しない)"
    vs_legacy: irrelevant
    counterexample: "誤実装: ↓最新 FAB を 閲覧モードのときだけ表示 する条件にする。入力モードで scrollback 中でも出ないため可視 assertion で fail する (states/assumptions の mode 非依存契約に反する)。"
  - id: UAC-016
    flow_ref: F-005
    given: "モバイル gate true・fontSize が default 14px・グリッド行数 R0 を観測済み"
    when: ".xterm-viewport で 1 回の連続 pinch out (指間距離を約 1.5 倍に) を行い touchend する"
    then: ".xterm .xterm-rows の computed font-size が 14px より大きく 28px 以下、かつ 18px 以上 (比率追従の証拠) で、表示行 (.xterm-rows の row 要素数) が R0 より減る (refit でグリッド再計算)"
    vs_legacy: must-fail
    counterexample: "誤実装A (非比例): 2 指距離の比率を無視し方向だけ見て ±2px ステップで動かす → 14→16 で 18px 未満となり 18px 以上 assertion で fail (比率追従していないことを判別)。誤実装B (refit 欠落): fontSize は変えるが fit() を呼ばず行数が R0 のまま → 行数減少 assertion で fail。legacy は pinch handler が無く font 不変・行数不変なので必ず fail する。"
  - id: UAC-017
    flow_ref: F-005
    given: "モバイル gate true・fontSize が min 境界 8px"
    when: "さらに pinch in (指を狭める) を行う"
    then: ".xterm .xterm-rows の computed font-size が 8px のまま (8px 未満にならない)"
    vs_legacy: irrelevant
    counterexample: "誤実装: clamp 下限を入れず比率をそのまま掛ける。font-size が 8px 未満 (例 5px) になり読めなくなる/0 以下で fit() が NaN cols を吐くため、下限 8px 維持 assertion で fail する。"
  - id: UAC-018
    flow_ref: F-005
    given: "モバイル gate true・pinch out で fontSize を 20px に変更し touchend 済み"
    when: "ページをリロードして同セッションを再描画する"
    then: "localStorage の web.term.fontSize が \"20\" で、再描画後の .xterm .xterm-rows の computed font-size が 20px"
    vs_legacy: irrelevant
    counterexample: "誤実装: fontSize を React state のみ保持し localStorage に書かない。リロードで default 14px に戻り localStorage 値が null なので、永続値 assertion と復元 font-size assertion の両方で fail する。"
  - id: UAC-019
    flow_ref: F-005
    given: "モバイル gate true・localStorage の web.term.fontSize に範囲外/破損値 (例 \"999\") を仕込んでリロード"
    when: "セッションが再描画される"
    then: "復元後の .xterm .xterm-rows の computed font-size が 28px (max にクランプ) で、NaN cols や極大 font にならない"
    vs_legacy: irrelevant
    counterexample: "誤実装: 読み出し値を [8,28] に clamp/検証せず term.options.fontSize に直流する。999px で描画され読めない/グリッドが破綻するため、28px クランプ assertion で fail する。"
  - id: UAC-020
    flow_ref: F-005
    given: "モバイル gate true・font-size 制御 (aria-label=\"文字サイズ\") が可視で fontSize=14px"
    when: "支援技術 (VoiceOver/TalkBack) または keyboard で ＋ ボタンを activate する"
    then: "fontSize が 1 ステップ増えて refit され、＋ ボタンは role=button・getBoundingClientRect 44×44px 以上・非空 aria-label を持つ"
    vs_legacy: irrelevant
    counterexample: "誤実装: fontSize 変更を 2 本指 pinch handler だけに実装し、非 pinch の代替コントロールを出さない。＋ ボタンが DOM に存在せず activate できないため、ボタン存在/サイズ assertion で fail する (VoiceOver/TalkBack は 2 指ジェスチャを自前コマンドに奪うため fontSize へ到達不能)。"
  - id: UAC-021
    flow_ref: F-006
    given: "viewport 1280px・pointer:fine (matchMedia gate false)・セッション可視"
    when: "初期描画後に DOM を観測する"
    then: "aria-label=\"キーボードを開く\"/\"キーボードを閉じる\"・aria-label=\"最新へスクロール\"・aria-label=\"文字サイズ\" の要素がいずれも DOM に存在せず、terminal-host ラッパに data-input-active 属性が無い"
    vs_legacy: must-pass
    counterexample: "誤実装: FAB を常時 render し coarse のとき以外 CSS display:none で隠す。要素が DOM に残り querySelector が当たるため 存在しない assertion で fail する (PC ユーザーの a11y tree に無意味なボタンが漏れる)。"
  - id: UAC-022
    flow_ref: F-006
    given: "viewport 700px・pointer:fine (narrow desktop window + マウス, 幅は ≤767px だが coarse でない)"
    when: "terminal 表示領域を mouse で click する"
    then: "document.activeElement === .xterm-helper-textarea かつ readonly 属性が無く、キーボード FAB が DOM 不在で terminal-host に data-input-active 属性が無い (= desktop 扱い)"
    vs_legacy: must-pass
    counterexample: "誤実装: gate を matchMedia (max-width: 767px) の 幅のみ (pointer 無視) で実装する。700px+fine が mobile 化し data-input-active 分離 + helper textarea readonly に入り、PC ユーザーが click しても入力できなくなる。activeElement assertion と readonly 無し assertion で fail する (narrow desktop window の PC regression を、幅のみ gate なら全モバイル/PC scenario が green のまま見逃す)。"
  - id: UAC-023
    flow_ref: F-006
    given: "viewport 1280px・pointer:fine・scrollTop を末尾に設定"
    when: "terminal 上で wheel up イベントを送る"
    then: ".xterm-viewport.scrollTop が減少し scrollback が遡れる"
    vs_legacy: must-pass
    counterexample: "誤実装: touch-action やカスタム scroll ハンドラを全環境に適用し wheel を握る。wheel で scrollTop が動かなくなり、減少 assertion で fail する。"
  - id: UAC-024
    flow_ref: F-007
    given: "モバイル gate true・閲覧モードでキーボード FAB が可視"
    when: "キーボード FAB 要素を観測する"
    then: "role=button かつ getBoundingClientRect().width≥44 かつ height≥44 かつ非空の aria-label を持つ"
    vs_legacy: must-fail
    counterexample: "誤実装: 32px アイコンボタン + aria-label 省略。width/height が 44 未満で aria-label が空のため、サイズ assertion と aria-label 非空 assertion で fail する (FR-A11Y-001 違反)。legacy には FAB 自体が無く計測対象が存在しないため必ず fail する。"
  - id: UAC-025
    flow_ref: F-007
    given: "モバイル gate true・閲覧モードで terminal-host の getBoundingClientRect を T0 として記録済み"
    when: "キーボード FAB tap → ↓最新 FAB 出現まで状態を変化させる"
    then: "terminal-host の getBoundingClientRect が T0 と一致し続ける (FAB 出現/状態変化で terminal box が縮まない=overlay 配置である)"
    vs_legacy: must-pass
    counterexample: "誤実装: FAB を terminal-host の flex 兄弟として通常フローに挿入する。FAB 出現時に flex:1 1 0 の terminal-host が残余を奪われ box が縮み height が変わるため、box 不変 assertion で fail する (ADR-0029/0065 違反)。"
  - id: UAC-026
    flow_ref: F-007
    given: "モバイル gate true・入力モードでキーボード FAB が aria-pressed=\"true\""
    when: "閲覧モードへ戻す (再 tap or blur)"
    then: "同じ FAB の aria-pressed が \"false\" に同期し aria-label が \"キーボードを開く\" に戻る"
    vs_legacy: irrelevant
    counterexample: "誤実装: aria-pressed を初期 render 時しか設定せず state 変化に同期しない。入力→閲覧後も aria-pressed=\"true\" が残り、screen reader が誤った状態を読むため false 同期 assertion で fail する。"
reference_ux:
  - name: "modal editor 流の閲覧/挿入 mode separation"
    stance: modeled_on
    aspects:
      - "閲覧 (copy/scroll) と入力 (insert) を別モードにし、デフォルトは閲覧で入力は明示遷移が必要"
      - "閲覧中はスクロール操作がそのままスクロールに割り当てられる (キー/スワイプが入力に吸われない)"
      - "モバイルの 軽く確認 ユースに合致 (フル CLI 操作は対象外)"
  - name: "モバイル SSH/コードエディタの キーボード toggle ボタン (Termius / a-Shell)"
    stance: modeled_on
    aspects:
      - "仮想キーボードの表示/非表示を専用ボタンで明示制御し、表示領域の半減をユーザーが選べる"
      - "aria-pressed でトグル状態を表現し、押下で focus/blur を切り替える"
  - name: "チャットアプリの jump-to-latest FAB (Slack/Telegram の ↓最新へ)"
    stance: modeled_on
    aspects:
      - "スクロール位置が末尾でないときだけ出現する floating ボタン"
      - "tap で最下部 (最新行) へスクロールし、到達後は自動的に消える"
      - "overlay 配置でコンテンツ flow を押し下げない"
  - name: "Material Design Floating Action Button primitive"
    stance: modeled_on
    aspects:
      - "コンテンツに重なる position:absolute の浮遊ボタンで、下地の terminal-host flex layout (ADR-0029) を変更しない"
      - "44×44px 以上のタッチターゲット (FR-A11Y-001) と固定スタック順 (複数 FAB の重なり回避)"
  - name: "iOS inputAccessoryView / keyboard-aware sticky toolbar (visualViewport API)"
    stance: modeled_on
    aspects:
      - "仮想キーボード上端に貼り付くツールバーのように、入力モード中 FAB をキーボード直上へ持ち上げ退出 affordance を常時到達可能に保つ"
  - name: "WAI-ARIA live region (aria-live=polite) status pattern"
    stance: modeled_on
    aspects:
      - "ジェスチャ起点でない状態変化 (OS キーボード閉じ→閲覧復帰、↓最新 FAB 出現) を screen reader に polite に通知する"
  - name: "desktop xterm の tap/click=即 focus=即キーボード挙動"
    stance: rejected
    aspects:
      - "モバイルでは tap が必ずキーボードを出すと表示領域が半減し scrollback 確認 (主ユース) ができないため不採用"
      - "PC (pointer:fine) では従来どおり採用するので、不採用はモバイル gate 内に限定する"
  - name: "専用モバイルビュー (要件の方針 c, 別レイアウトの全置換)"
    stance: rejected
    aspects:
      - "非ゴール明記。既存 TerminalPane / terminal-slot 構造を保ったまま overlay 追加と touch-action 付与だけで実現するため別ビューは作らない"
  - name: "ピンチズームを OS のブラウザズーム (viewport scale) に委ねる"
    stance: rejected
    aspects:
      - "ブラウザズームは xterm のグリッド (cols/rows) を refit せず文字がぼやけ scrollback 行折り返しが狂うため不採用。term.options.fontSize + refit でグリッドを保つ"
legacy_context:
  source_implementation: "src/client/web/src/components/TerminalPane.tsx (163 行, xterm.js 5.5.0 + @xterm/addon-fit のみ)。render は単一 <div ref={hostRef} className=terminal-host /> を返すだけ。.terminal-host は app.css の flex:1 1 0 / height:var(--dvh) / 全画面占有。touch-action / pointer media / mode 概念は一切なし。host は .terminal-slot (view.css absolute inset:0 overlay, ADR-0065) 内に常時マウント。scrollback は server-side VT バッファ供給 (ADR-0066, xterm 側 scrollback:10000)、fit は ResizeObserver + rAF coalesce (ADR-0034)、theme は useXtermTheme (ADR-0059)、subscribe 所有権は keyed remount (ADR-0030)。"
  inherited_behaviors:
    - "PC (pointer:fine) では tap/click が xterm helper textarea を focus し従来どおり入力受付 (mobile gate に該当しない環境は一切挙動を変えない)"
    - "term.onData → conn.send k:i の入力経路、term.onResize → conn.send k:r の resize 経路"
    - "conn.onOutput の base64→Uint8Array バイト忠実 write"
    - "xterm scrollback:10000 と ADR-0066 の 2 段 seed frame による late-join 履歴表示"
    - "ADR-0034 の scheduleFit (rafPending ガード付き単一 rAF coalesce)"
    - "ADR-0030 keyed remount による 1 session 1 TerminalPane インスタンスの subscribe 所有権"
    - "ADR-0059 useXtermTheme による data-theme 連動 ITheme 再適用"
    - "ADR-0065 terminal-slot=absolute inset:0 overlay / data-active 切替 / .terminal-host flex:1 1 0"
  replaced_behaviors:
    - "モバイル (max-width:767px AND pointer:coarse) では tap が helper textarea へ focus を移さない (focus 移動をブロックして仮想キーボードを出さない) — 閲覧モード既定。readonly は defense-in-depth として保持"
    - "モバイル閲覧では .xterm-viewport の touch swipe で scrollback をスクロールでき (touch-action:pan-y)、long-press 起点のドラッグでは文字選択を維持する (swipe/scroll と long-press/選択を dwell で判別)"
    - "モバイルでは入力モードへの遷移を キーボード FAB tap のみで行い、FAB は focus を奪わず enter と同時の blur-exit を起こさない"
    - "入力モード退出は FAB 再 tap / OS キーボード非表示 (blur) / Esc / terminal コンテンツ部の tap (領域外タップ) の 4 経路で data-input-active=false に同期し aria-live で告知"
    - "scrollback 中 (閲覧/入力 mode 非依存) だけ ↓最新 FAB を表示し tap で末尾へスクロール (reduced-motion 尊重)"
    - "2 本指 pinch で fontSize を比率連続追従 + min8/max28 clamp で変更し refit、device-scoped localStorage に永続化、復元時も clamp 検証。pinch 不能ユーザー向けに font-size 制御 (＋/－/リセット) を代替提供"
    - "入力モード中、FAB 群を visualViewport 上端より上に保つ (iOS soft keyboard で入力行/退出 affordance が隠れるのを防ぐ)"
states:
  - "desktop モード (gate false: 幅>767px もしくは pointer:fine): FAB/ font-size 制御 無し、click=focus、wheel scroll、選択コピー (legacy 完全維持)"
  - "mobile-閲覧モード (gate true, data-input-active=false): tap で keyboard 出ない / helper textarea readonly / swipe scroll 可 / long-press 選択可 / キーボード FAB aria-pressed=false 表示 / 初回のみ coachmark"
  - "mobile-入力モード (gate true, data-input-active=true): helper textarea focus & readonly 解除 / 仮想キーボード表示 / キーボード FAB aria-pressed=true / FAB 群は visualViewport 上端より上に追従"
  - "↓最新 FAB 非表示 (scrollTop=末尾 ±2px): DOM 不在 or hidden"
  - "↓最新 FAB 表示 (scrollTop≠末尾, 閲覧/入力どちらでも scrollback 中): aria-label=最新へスクロール button 可視"
  - "pinch 操作中: term.options.fontSize を 8–28px clamp で連続変更 + scheduleFit 経由 refit + 中央に transient fontSize indicator 表示"
  - "fontSize 永続化済み: localStorage web.term.fontSize に確定値、次回 device で clamp 検証して復元"
  - "FAB スタック: 下=キーボード FAB → 上=↓最新 FAB を 8px gap、font-size 制御は別位置、既存 .notification-toast と z-index/位置を分離"
  - "aria-live status: 非ジェスチャ起点の状態変化 (blur→閲覧復帰 / ↓最新 FAB 出現) を polite に読み上げ"
edge_cases:
  - "alt-screen 全画面プログラム (vim/less) 中: ADR-0066 で scrollback が空のため ↓最新 FAB は出ない (scrollTop=末尾固定)。閲覧の swipe も移動量 0。alt-screen 終了後は scrollback 復活で FAB/swipe が機能する"
  - "セッション未選択 (sessionId=null): TerminalPane は host を出すが入力経路 drop。入力モード FAB tap で focus しても term.onData が sid 無しで drop する (legacy 踏襲) — FAB は表示してよいが no-op"
  - "iPadOS Safari が pointer:fine を報告するケース: gate が false になり desktop 扱い。幅 AND coarse の AND 条件で意図通り (タッチ iPad でも外付けキーボード/トラックパッド時は desktop UX)"
  - "デバイス回転で 767px 境界をまたぐ: matchMedia の change を購読し mode を再評価。入力モード中に desktop へ移ったら入力モード state を破棄し readonly を解除 (desktop は readonly 無し)"
  - "iOS Safari の soft keyboard は layout viewport / 100dvh を縮めず overlay するため ResizeObserver は発火しない。入力行と退出 FAB がキーボード裏に隠れるのを防ぐため、visualViewport の resize/scroll を購読して FAB 群 bottom を visualViewport.height + offsetTop 基準で持ち上げ、入力行可視を保つ (この経路の実機検証は Open Questions, Playwright では実キーボードが立たない)"
  - "入力モード遷移時の iOS focus-zoom: iOS Safari は font-size<16px の textarea を focus するとページを自動ズームする。helper textarea (.xterm-helper-textarea) の font-size はグリッドの term.options.fontSize と独立に常時 16px 以上へ固定し、入力モード突入時の意図しない viewport zoom を防ぐ (グリッド描画は .xterm-rows 側なので表示には影響しない)。ピンチの grid fontSize は 8–28px clamp のまま維持"
  - "ピンチ中に theme 変更 (data-theme): ADR-0059 の useXtermTheme は options.theme のみ更新し fontSize に触れないため衝突しない。fontSize 永続値は theme と独立キー"
  - "ピンチで fontSize 変更直後に server から大量 output: scheduleFit の rafPending ガード (ADR-0034) で fit() が単一 rAF に coalesce され、refit と write が競合しない"
  - "scrollback 末尾判定の閾値: scrollTop が scrollHeight-clientHeight にちょうど一致しない端数 (sub-pixel) で FAB がチラつくのを防ぐため近接 (±2px) を末尾扱いとする"
  - "FAB/↓最新スクロールの motion: prefers-reduced-motion:reduce では smooth scroll/fade を即時化する。新規 animation は ADR-0064 の view.css 末尾 @media (prefers-reduced-motion: reduce) 単一 guard ブロックに追記する (集約先を分散させない)"
  - "FAB の safe-area 二重計上回避: .app-shell が四辺に env(safe-area-inset-*) を既適用 (FR-LAYOUT-004) のため、FAB は env(safe-area-inset) を再加算せず親 inset 内に 16px offset で置く。notched 端末で過剰に内側へ寄らない"
  - "localStorage が無効/容量超過 (private mode): fontSize 永続化が失敗しても default 14px で動作継続 (try/catch で握りつぶし、UX は degrade のみ)"
  - "localStorage の不正/範囲外 fontSize 値: 復元時に [8,28] へ clamp 検証し、parse 不能/範囲外は default 14px にフォールバックする (NaN cols / 読めない font の起動時再現を防ぐ)"
  - "1 本指 + 後から 2 本目で pinch 開始: touchstart の touches.length 遷移を扱う。1 指 swipe (scroll) と 2 指 pinch を touches.length で分岐し誤判定でモード遷移しない"
  - "coachmark の抑止: 初回閲覧モード突入でのみ表示し、tap または数秒で dismiss。localStorage web.term.hintSeen で 2 回目以降は出さない (認知ノイズを足さない)"
assumptions:
  - "モバイル gate の確定値: matchMedia (max-width: 767px) and (pointer: coarse) が true のときのみモバイル UX を有効化する。既存 app.css の @media (max-width:767px) と幅境界を一致させ、pointer:coarse の AND で narrow desktop window (pointer:fine) を除外して PC 非影響を保証する"
  - "pointer media は CSS の @media (pointer:coarse) と JS の matchMedia の両輪で判定し、JS 側 (FAB の render 有無) を真実源にする (CSS display 切替ではなく条件 render で a11y tree から除外)"
  - "閲覧モードで tap がキーボードを出さない load-bearing 機構は helper textarea への focus 移動をブロックすること。readonly 属性は defense-in-depth (万一 focus されても無キーボード) として閲覧モードで付与し、入力モードで外す。両者は矛盾しない (未 focus の readonly textarea は無キーボードで整合)。focus-block の実装手段は plan-how が確定する"
  - "ピンチズーム font size の確定値: default=14px, min=8px, max=28px, 比率連続追従 (整数 px 丸め)。default 14 は既存 desktop の体感に近く、min 8 で全体俯瞰、max 28 で確認応答時の可読性を確保"
  - "fontSize 永続化方針の確定: device-scoped (per-browser) で localStorage キー web.term.fontSize に保存。per-session ではない (同一デバイスの好みは全セッション共通が自然、web-active-session-ownership の教訓どおり session に紐付けない)。復元時は [8,28] clamp 検証"
  - "入力モード退出条件の確定: (1) キーボード FAB 再 tap (aria-pressed toggle), (2) helper textarea の blur (OS のキーボード非表示), (3) hardware keyboard の Esc keydown, (4) terminal コンテンツ部の tap (領域外タップ)。この 4 つで data-input-active=false へ同期し aria-live で告知する"
  - "helper textarea の font-size は grid の term.options.fontSize と独立に常時 16px 以上へ固定し、iOS の focus-zoom を抑止する (グリッド表示は .xterm-rows 側で別系統)"
  - "FAB は terminal-slot の absolute overlay 子として配置し、env(safe-area-inset) を再加算せず親 inset 内 16px offset で置く (ADR-0065 overlay 原則 + FR-LAYOUT-004 single-source safe-area)。複数 FAB は固定スタック順 (下:キーボード FAB → 上:↓最新 FAB, 8px gap)、既存 .notification-toast と z-index/位置を分離する"
  - "swipe scrollback は .xterm-viewport に touch-action:pan-y を付与して browser ネイティブ scroll に委ねる。文字選択は long-press dwell (約 500ms) を起点に xterm の選択へ分岐する。標準 API での成立可否は Open Questions で plan-how に委ねる (addon 追加せず標準 API 優先)"
  - "↓最新 FAB は閲覧/入力の mode に依存せず scrollback 中 (scrollTop≠末尾) なら表示する (入力モード中でも過去を見ていれば最新復帰したい需要があるため)"
  - "pinch の代替コントロール (font-size 制御 ＋/－/リセット) は VoiceOver/TalkBack で到達可能な role=button・44px・aria-label 付き要素として提供する (2 指ジェスチャは支援技術が奪うため)"
  - "FAB の視覚は既存トークン (--accent / --surface-*) と ADR-0059 theme 連動を用い、既存アイコンボタン (SessionDrawer close / CommandSearchTrigger) のスタイル言語に合わせる (ad-hoc な色/サイズを新設しない)"
  - "ATDD は Playwright で pointer:coarse + viewport サイズを emulate し、touch swipe/pinch/long-press を合成イベントで再現する。font-size は computed style、focus は document.activeElement と focus イベント数、選択は term.getSelection()、mode は data 属性、FAB は aria-label/getBoundingClientRect で観察する。実 soft keyboard を要する観察 (visualViewport 連動) は実機検証に回す"
tags: [web-ui, terminal, mobile, accessibility, refactor]
source_paths:
  - src/client/web/src/components/TerminalPane.tsx
  - src/client/web/src/css/app.css
  - src/client/web/src/css/view.css
relations: []
---

# UX — Web UI ターミナルタブ (TerminalPane) のモバイル UX 再設計

## Goal

Web UI のターミナルタブ (`src/client/web/src/components/TerminalPane.tsx`) は現状 xterm.js 5.5.0 + addon-fit のみの PC 前提 UX で、`.terminal-host` が `height:var(--dvh)` で全画面を占有し touch-action 等のタッチ向け指定を持たない。スマホでは (1) tap で仮想キーボードが必ず出て表示領域が半減する、(2) `.xterm-viewport` がタッチ swipe で scrollback できない、(3) アドレスバーを動かす隙間もない、という 3 つの問題が主ユース (late-join した既存セッションの状況確認 / 軽い確認応答) を阻害している。

本 UX 再設計は**モバイル (スマホ・タブレット) のみ**を対象に、閲覧モード (デフォルト) と入力モードを明示分離する。閲覧モードでは tap でキーボードを出さず、指スワイプで scrollback をスクロールし、long-press で文字選択を維持する。入力モードはキーボード FAB の明示 tap でのみ立ち上がり、FAB 再 tap / OS のキーボード非表示 (blur) / Esc / 領域外タップの 4 経路で退出する。scrollback 中だけ ↓最新 FAB を出し、2 本指ピンチ (+ 支援技術向けのステッパ代替) で fontSize を変えて refit する。PC (pointer:fine) は keyboard 入力 / mouse wheel / ドラッグ選択コピーを含めて**完全に現状維持**で、モバイル UX は `matchMedia('(max-width: 767px) and (pointer: coarse)')` という幅 AND pointer:coarse の gate 内に限定する。FAB は terminal-slot の absolute overlay 子として配置し、terminal-host の flex layout (ADR-0029) と box サイズに干渉しない。ADR-0030 (keyed remount) / ADR-0034 (refit rAF coalesce) / ADR-0059 (theme) / ADR-0064 (reduced-motion 単一 guard) / ADR-0065 (terminal-slot overlay) / ADR-0066 (scrollback) はすべて維持する。

**Reference UX**:
- *Modeled on*: modal editor 流の閲覧/挿入 mode separation (デフォルト閲覧・入力は明示遷移)、Termius / a-Shell のキーボード toggle ボタン (表示領域半減をユーザーが選ぶ)、Slack / Telegram の jump-to-latest FAB (末尾でないときだけ出現し到達後消える)、Material Design FAB primitive (overlay 浮遊・44px・固定スタック順)、iOS inputAccessoryView / keyboard-aware sticky toolbar (入力モード中 FAB を visualViewport 上端より上へ追従)、WAI-ARIA live region (非ジェスチャ起点の状態変化を polite に通知)。
- *Rejected*: desktop xterm の tap=即キーボード挙動 (モバイルでは表示領域半減で主ユースを壊す。PC では従来どおり採用し不採用はモバイル gate 内に限定)、専用モバイルビュー (要件の方針 c、非ゴール。既存構造を保ち overlay + touch-action だけで実現)、OS ブラウザズームへの委譲 (xterm グリッドが refit されず文字がぼやけ折り返しが狂う。fontSize + refit でグリッドを保つ)。

**Migration Context** (refactor: 現状 PC 専用 TerminalPane.tsx のモバイル UX 再設計):
- *Source implementation*: `src/client/web/src/components/TerminalPane.tsx` (163 行, xterm.js 5.5.0 + @xterm/addon-fit のみ)。render は単一 `<div ref={hostRef} className="terminal-host" />`。`.terminal-host` は `flex:1 1 0 / height:var(--dvh)` で全画面占有。touch-action / pointer media / mode 概念は無し。scrollback は server-side VT バッファ (ADR-0066)、fit は ResizeObserver + rAF coalesce (ADR-0034)、theme は useXtermTheme (ADR-0059)、subscribe 所有権は keyed remount (ADR-0030)、host は terminal-slot の absolute overlay (ADR-0065) 内に常時マウント。
- *Inherited (現状維持)*: PC の tap/click=focus 入力受付、term.onData/onResize の送信経路、base64 バイト忠実 write、ADR-0066 の late-join 履歴、ADR-0034 scheduleFit、ADR-0030 keyed remount、ADR-0059 theme 連動、ADR-0065 terminal-slot overlay。
- *Replaced (モバイル gate 内のみ)*: tap で textarea へ focus を移さない閲覧モード既定 (readonly は defense-in-depth)、touch swipe での scrollback + long-press 文字選択維持、キーボード FAB のみでの入力モード遷移 (FAB は focus を奪わない)、4 経路退出 + aria-live 告知、mode 非依存の ↓最新 FAB、pinch + ステッパでの fontSize 変更 + clamp + 永続化 + 復元時 clamp、入力モード中の FAB の visualViewport 追従。

## Target Users

- **スマホから既存セッションに late-join して状況確認したい運用者** — 主ユースは scrollback で過去出力を遡ること。tap で毎回キーボードが出て表示が半減するのが最大の障害。主要 modality は pointer-touch (swipe / long-press)。
- **スマホ/タブレットから確認応答・short input だけ軽く打ちたい運用者** — y/n や 1 行コマンドを「打ちたいときだけ」打つ。フル CLI 操作は Termius/Blink 前提で本タスク対象外。主要 modality は pointer-touch + 入力モード時の software keyboard。
- **PC で keyboard / mouse wheel / ドラッグ選択コピーを使う既存ユーザー** — デスクトップ端末の業界標準体験で問題なし。**現状維持が要件**。modality は keyboard + pointer-mouse。narrow な desktop window (≤767px 幅) でも pointer:fine なら desktop 扱いであること。
- **VoiceOver/TalkBack を使うモバイル支援技術ユーザー** — FAB は 44px + aria-label 必須。2 指ピンチは支援技術が自前ジェスチャに奪うため、fontSize 変更には非 pinch の代替 (ステッパ) が必要。非ジェスチャ起点の mode 変化は aria-live で受け取りたい。modality は screen-reader。

## Primary Flows

### F-001 モバイル mode 分離 — 閲覧モード既定と入力モードへの明示遷移/退出

**狙い**: モバイル gate 内で閲覧モードを既定にし、入力モードへの遷移を「キーボード FAB tap のみ」に縛る。退出は FAB 再 tap / blur / Esc / 領域外タップの 4 経路で同期する。enter と同時の blur-exit race、領域外タップ、非ジェスチャ起点の mode 変化通知まで含めて、入力 affordance が常に到達可能であることを担保する。
**前提**: gate true (≤767px AND coarse)。helper textarea への focus-block が閲覧モードのキーボード抑止の load-bearing 機構 (readonly は defense-in-depth)。FAB は pointerdown を抑止し focus を奪わない。

### F-002 閲覧モードの touch swipe による scrollback スクロール

**狙い**: `.xterm-viewport` に touch-action:pan-y を付与し、縦スワイプを browser ネイティブ scroll に委ねる。swipe が入力モードに化けず focus も奪わないことを観測する。
**前提**: 閲覧モード。scrollback に画面 2 枚分以上の行。標準 viewport が scrollTop を持つ。

### F-003 閲覧モードの文字選択維持 — long-press 起点の選択は swipe/scroll と判別

**狙い**: 元要件の「閲覧モードで文字選択は維持」を満たす。touch-action:pan-y で swipe=scroll に割り当てつつ、long-press dwell (約 500ms) を起点としたドラッグは scroll ではなく文字選択に分岐し、入力モードにも遷移しない。swipe と選択を dwell で判別する。
**前提**: 閲覧モード。選択可能な文字列が描画済み。`否定役 blocker-1 の機構衝突 (pan-y と選択の両立)` を long-press 分岐で解消する。

### F-004 ↓最新 FAB — scrollback 中 (mode 非依存) のみ表示し末尾へ復帰

**狙い**: scrollTop が末尾でないときだけ ↓最新 FAB を overlay 表示し、tap で末尾へ。閲覧/入力の mode に依存しない。末尾到達後は自動的に消える。reduced-motion を尊重する。
**前提**: gate true。末尾判定は ±2px 近接を末尾扱い (sub-pixel チラつき回避)。

### F-005 fontSize 変更 — 2 本指ピンチ + 支援技術向けステッパ (clamp + 永続化)

**狙い**: 2 指間距離の比率に連続追従して fontSize を変え、min8/max28/default14 に clamp し、ADR-0034 scheduleFit 経由で refit する。確定値を device-scoped localStorage に永続化し、復元時も clamp 検証する。pinch を奪われる支援技術ユーザー向けに ＋/－/リセットの非 pinch 代替を提供する。pinch 中は現在 fontSize を transient 表示し、indicator tap で default へ reset する。
**前提**: gate true。touches.length=2 で pinch、1 で swipe を判別。

### F-006 PC 非影響 — pointer:fine 環境で legacy 挙動を完全維持 (narrow-width AND coarse 判別)

**狙い**: gate false のとき (幅>767px もしくは pointer:fine) はモバイル overlay を一切 render せず、click=focus / wheel scroll / 選択コピーを完全維持する。とくに **narrow な desktop window (≤767px 幅 + pointer:fine)** が幅のみ gate の誤実装で mobile 化しないことを判別する。
**前提**: gate は幅 AND coarse の AND。本フローは PC 保護フローのため全シナリオが `must-pass` (vs_legacy semantics 上正しい。詳細は Open Questions)。

### F-007 アクセシビリティ — FAB の 44px / aria / overlay box 不変 / token 整合

**狙い**: 全 FAB が 44×44px + 非空 aria-label を持ち、キーボード FAB は aria-pressed を state に同期する。FAB は terminal-host の box を縮めない overlay 配置で、safe-area を二重計上せず、複数 FAB が重ならず、既存トークン/theme と整合する。
**前提**: gate true。FR-A11Y-001 (44px) / FR-LAYOUT-004 (safe-area single-source) / ADR-0029・0065 (overlay) / ADR-0059 (theme) を遵守。

## Acceptance Scenarios

### F-001

**UAC-001**
- Given: モバイル gate (max-width: 767px and pointer: coarse) が true の環境でセッションを開き terminal-slot[data-active=true] が可視
- When: セッション初期描画が完了する
- Then: terminal-host ラッパに `data-input-active="false"` 属性があり、`document.activeElement` は `.xterm-helper-textarea` ではなく、`.xterm-helper-textarea` に `readonly` 属性が存在する
- **Counterexample**: モバイル判定を入れず legacy のまま render すると `data-input-active` 属性が DOM に無く attribute assertion が undefined で fail し、legacy は readonly を付与しないため readonly assertion でも fail する。
- **vs Legacy**: must-fail

**UAC-002**
- Given: モバイル gate true・閲覧モード (`data-input-active="false"`)・helper textarea に focus イベントリスナを装着済み
- When: xterm 表示領域の中央座標を tap (touchstart→touchend) する
- Then: tap 完了後 `document.activeElement` は依然 `.xterm-helper-textarea` でなく、tap 中に textarea へ focus イベントが 1 度も dispatch されない
- **Counterexample**: touchend で `term.focus()` 直後に `setTimeout(()=>term.blur())` する「チラ見せ」方式は focus イベントが実 dispatch されるため focus 発火数 0 の assertion で fail する。(初稿の `visualViewport.height` 不変 assertion は Playwright で実キーボードが立たず常に真=判別力ゼロなので採用せず、focus 発火数のみを判別軸にした。)
- **vs Legacy**: must-fail

**UAC-003**
- Given: モバイル gate true・閲覧モード・キーボード FAB が `aria-pressed="false"` で可視
- When: キーボード FAB を tap し、tap 直後 200ms 経過まで観測する
- Then: terminal-host が `data-input-active="true"`、`document.activeElement === .xterm-helper-textarea` かつ helper textarea に readonly 無し、FAB `aria-pressed="true"` / `aria-label="キーボードを閉じる"`、200ms 後も `data-input-active` が `"true"` のまま (enter→即 exit の flicker が無い)
- **Counterexample**: (A) readonly を外し忘れると iOS が readonly textarea でキーボードを出さず readonly 不在 assertion で fail。(B) FAB が pointerdown で focus を奪い blur-exit listener が発火すると enter 直後に `false` へ戻り 200ms 後の `"true"` 維持 assertion で fail (enter/exit race)。
- **vs Legacy**: irrelevant

**UAC-004**
- Given: モバイル gate true・入力モード (`data-input-active="true"`, `aria-pressed="true"`)
- When: 同じキーボード FAB を再 tap する
- Then: `data-input-active="false"` に戻り、`document.activeElement` が `.xterm-helper-textarea` でなくなり、readonly が再付与され、FAB `aria-pressed="false"`
- **Counterexample**: FAB を単発トリガーにし toggle にしない (常に focus を試みる) と再 tap 後も `"true"` のままで false 遷移 assertion で fail する。
- **vs Legacy**: irrelevant

**UAC-005**
- Given: モバイル gate true・入力モード・helper textarea が focus 済み
- When: terminal の表示コンテンツ部 (helper textarea 以外) を tap する (領域外タップ相当)
- Then: `data-input-active="false"` になり FAB `aria-pressed="false"` に同期する
- **Counterexample**: outside-tap を購読せず xterm がコンテンツ tap で textarea を再 focus するだけにすると `data-input-active="true"` のまま残り false assertion で fail する (要件「領域外タップで閉じる」未達)。
- **vs Legacy**: irrelevant

**UAC-006**
- Given: モバイル gate true・入力モード・helper textarea が focus 済み
- When: helper textarea を programmatic に blur する (OS のキーボード非表示=blur 相当) もしくは Esc keydown を送る
- Then: `data-input-active="false"` になり FAB `aria-pressed="false"` に同期し、aria-live=polite 領域に「閲覧モードに戻りました」のテキストが現れる
- **Counterexample**: 入力モード state を FAB クリックハンドラだけで管理し blur/Esc を購読しないと OS がキーボードを閉じても `"true"` の幽霊状態が残り、false assertion と aria-live assertion で fail する。
- **vs Legacy**: irrelevant

### F-002

**UAC-007**
- Given: モバイル gate true・閲覧モード・scrollback に画面 2 枚分以上・`scrollTop` が最大値 (末尾)
- When: `.xterm-viewport` 上で下から上へ 200px の touch swipe を行う
- Then: `scrollTop` がスワイプ前より小さくなり、`document.activeElement` は `.xterm-helper-textarea` でなく `data-input-active="false"` のまま
- **Counterexample**: touch-action を付けず legacy のままだと `.xterm-viewport` が `touch-action:auto` を継承し xterm が touchmove を握る/body スクロールに化けて `scrollTop` が変化せず、scrollTop 減少 assertion で fail する。
- **vs Legacy**: must-fail

**UAC-008**
- Given: モバイル gate true・閲覧モード・`scrollTop` を最大値の半分に手動設定済み
- When: 上から下へ 200px の touch swipe を行う
- Then: `scrollTop` がスワイプ前より大きくなり末尾方向へ戻り、`data-input-active="false"` のまま
- **Counterexample**: touch swipe を JS で握って「1 swipe=入力モード移行」にすると `scrollTop` が動かず `data-input-active` が `"true"` になり、scrollTop 増加 assertion と false 維持 assertion の両方で fail する。
- **vs Legacy**: must-fail

**UAC-009**
- Given: モバイル gate true・閲覧モード・helper textarea に focus リスナ装着
- When: scrollback 領域で縦 swipe を 3 回連続で行う
- Then: swipe 中・後を通じ helper textarea へ focus イベントが 0 回、`data-input-active="false"` を維持する
- **Counterexample**: swipe の touchstart を tap と区別できず touchstart で focus すると swipe 開始時に focus が発火し `"true"` 化するため focus 発火数 0 assertion で fail する。
- **vs Legacy**: must-fail

### F-003

**UAC-010**
- Given: モバイル gate true・閲覧モード・terminal に選択可能な文字列が描画されている
- When: terminal コンテンツ上で long-press (約 500ms 静止) してからドラッグする
- Then: `term.getSelection()` が非空になり `.xterm-selection-layer` に選択矩形が可視で、かつ `document.activeElement` は `.xterm-helper-textarea` でなく `data-input-active="false"` のまま
- **Counterexample**: long-press を tap 同様に扱って `term.focus()` を呼ぶと選択範囲が生成されず textarea が focus され、選択非空 assertion と activeElement assertion で fail する。legacy は mode 概念が無く tap で textarea を focus する (= キーボードを出す) ため必ず fail する。
- **vs Legacy**: must-fail

**UAC-011**
- Given: モバイル gate true・閲覧モード・`scrollTop` が最大値 (末尾)
- When: long-press を伴わない 200px の縦 swipe を行う
- Then: `scrollTop` がスワイプ前より小さくなり、`term.getSelection()` は空 (選択は発生しない)
- **Counterexample**: touchstart 即 selection 開始 (long-press dwell 無視) にすると swipe で選択が走り `scrollTop` が動かず、scrollTop 減少 assertion と選択空 assertion の両方で fail する。legacy は touch-action 未指定で swipe scroll 自体が成立せず `scrollTop` 不変なので fail する。
- **vs Legacy**: must-fail

### F-004

**UAC-012**
- Given: モバイル gate true・`scrollTop` が末尾 (`scrollHeight-clientHeight` と ±2px 内)
- When: 初期描画後に DOM を観測する
- Then: `aria-label="最新へスクロール"` の要素が DOM に存在しない (または親が hidden 属性を持つ)
- **Counterexample**: ↓最新 FAB を常時 render し CSS `opacity:0` で隠すと要素が accessibility tree に残り querySelector で取得できるため「存在しない/hidden」assertion で fail する (screen reader にも幽霊ボタンとして読まれる)。
- **vs Legacy**: irrelevant

**UAC-013**
- Given: モバイル gate true・末尾に居て ↓最新 FAB 非表示
- When: 上方向 swipe で `scrollTop` を末尾から 300px 以上戻す
- Then: `aria-label="最新へスクロール"` の button が可視になり `getBoundingClientRect().width≥44` かつ `height≥44`
- **Counterexample**: scroll を監視せず「入力モードのとき表示」など別条件で出すと末尾離脱で出現せず可視 assertion で fail する。legacy には ↓最新 FAB 自体が無いため必ず fail する。
- **vs Legacy**: must-fail

**UAC-014**
- Given: モバイル gate true・`scrollTop` が末尾から離れ ↓最新 FAB 可視
- When: ↓最新 FAB を tap する
- Then: `scrollTop` が `scrollHeight-clientHeight` (末尾, ±2px) と一致し、その後 `aria-label="最新へスクロール"` の要素が再び DOM から消える/hidden になる
- **Counterexample**: tap で `term.scrollToBottom()` を呼ぶが FAB の表示条件を更新しないと `scrollTop` は末尾になるが FAB が出たままで「末尾後に消える」assertion で fail する。
- **vs Legacy**: irrelevant

**UAC-015**
- Given: モバイル gate true・入力モード (`data-input-active="true"`)・scrollback 中 (`scrollTop≠末尾`)
- When: DOM を観測する
- Then: `aria-label="最新へスクロール"` の button が可視である (FAB 表示は閲覧/入力の mode に依存しない)
- **Counterexample**: ↓最新 FAB を「閲覧モードのときだけ表示」条件にすると入力モードで scrollback 中でも出ず可視 assertion で fail する (mode 非依存契約に反する)。
- **vs Legacy**: irrelevant

### F-005

**UAC-016**
- Given: モバイル gate true・fontSize が default 14px・グリッド行数 R0 を観測済み
- When: `.xterm-viewport` で 1 回の連続 pinch out (指間距離を約 1.5 倍に) を行い touchend する
- Then: `.xterm .xterm-rows` の computed font-size が 14px より大きく 28px 以下、かつ **18px 以上** (比率追従の証拠) で、表示行数が R0 より減る (refit でグリッド再計算)
- **Counterexample**: (A 非比例) 比率を無視し方向だけで ±2px ステップにすると 14→16 で 18px 未満となり「18px 以上」assertion で fail (比率追従していないことを判別)。(B refit 欠落) `fit()` を呼ばないと行数が R0 のままで行数減少 assertion で fail。legacy は pinch handler が無く font/行数不変なので必ず fail する。
- **vs Legacy**: must-fail

**UAC-017**
- Given: モバイル gate true・fontSize が min 境界 8px
- When: さらに pinch in (指を狭める) を行う
- Then: `.xterm .xterm-rows` の computed font-size が 8px のまま (8px 未満にならない)
- **Counterexample**: clamp 下限を入れず比率を直掛けすると font-size が 8px 未満 (例 5px) になり読めなくなる/0 以下で `fit()` が NaN cols を吐くため、下限 8px 維持 assertion で fail する。
- **vs Legacy**: irrelevant

**UAC-018**
- Given: モバイル gate true・pinch out で fontSize を 20px に変更し touchend 済み
- When: ページをリロードして同セッションを再描画する
- Then: localStorage の `web.term.fontSize` が `"20"` で、再描画後の `.xterm .xterm-rows` の computed font-size が 20px
- **Counterexample**: fontSize を React state のみ保持し localStorage に書かないとリロードで default 14px に戻り localStorage が null なので永続値 assertion と復元 font-size assertion の両方で fail する。
- **vs Legacy**: irrelevant

**UAC-019**
- Given: モバイル gate true・localStorage の `web.term.fontSize` に範囲外/破損値 (例 `"999"`) を仕込んでリロード
- When: セッションが再描画される
- Then: 復元後の `.xterm .xterm-rows` の computed font-size が 28px (max にクランプ) で、NaN cols や極大 font にならない
- **Counterexample**: 読み出し値を `[8,28]` に clamp/検証せず `term.options.fontSize` に直流すると 999px で描画されグリッドが破綻するため、28px クランプ assertion で fail する。
- **vs Legacy**: irrelevant

**UAC-020**
- Given: モバイル gate true・font-size 制御 (`aria-label="文字サイズ"`) が可視で fontSize=14px
- When: 支援技術 (VoiceOver/TalkBack) または keyboard で ＋ ボタンを activate する
- Then: fontSize が 1 ステップ増えて refit され、＋ ボタンは `role=button`・`getBoundingClientRect` 44×44px 以上・非空 aria-label を持つ
- **Counterexample**: fontSize 変更を 2 本指 pinch handler だけに実装し非 pinch の代替を出さないと ＋ ボタンが DOM に無く activate できないため、ボタン存在/サイズ assertion で fail する (VoiceOver/TalkBack は 2 指ジェスチャを自前コマンドに奪うため fontSize へ到達不能)。
- **vs Legacy**: irrelevant

### F-006

**UAC-021**
- Given: viewport 1280px・pointer:fine (matchMedia gate false)・セッション可視
- When: 初期描画後に DOM を観測する
- Then: `aria-label="キーボードを開く"/"キーボードを閉じる"`・`aria-label="最新へスクロール"`・`aria-label="文字サイズ"` の要素がいずれも DOM に存在せず、terminal-host に `data-input-active` 属性が無い
- **Counterexample**: FAB を常時 render し coarse 以外で CSS `display:none` で隠すと要素が DOM に残り querySelector が当たるため「存在しない」assertion で fail する (PC ユーザーの a11y tree に無意味なボタンが漏れる)。
- **vs Legacy**: must-pass

**UAC-022**
- Given: viewport 700px・pointer:fine (narrow desktop window + マウス, 幅は ≤767px だが coarse でない)
- When: terminal 表示領域を mouse で click する
- Then: `document.activeElement === .xterm-helper-textarea` かつ readonly 無し、キーボード FAB が DOM 不在で terminal-host に `data-input-active` 属性が無い (= desktop 扱い)
- **Counterexample**: gate を `matchMedia('(max-width: 767px)')` の幅のみ (pointer 無視) で実装すると 700px+fine が mobile 化し `data-input-active` 分離 + readonly に入り、PC ユーザーが click しても入力できなくなる。activeElement assertion と readonly 無し assertion で fail する。**この誤実装は幅のみ gate のため全モバイル scenario (coarse+narrow を emulate) と PC scenario (1280px 幅) を green のまま通し、narrow desktop window の PC regression だけを取りこぼす** — UAC-022 が唯一の判別点。
- **vs Legacy**: must-pass

**UAC-023**
- Given: viewport 1280px・pointer:fine・`scrollTop` を末尾に設定
- When: terminal 上で wheel up イベントを送る
- Then: `.xterm-viewport.scrollTop` が減少し scrollback が遡れる
- **Counterexample**: touch-action やカスタム scroll ハンドラを全環境に適用し wheel を握ると `scrollTop` が動かず減少 assertion で fail する。
- **vs Legacy**: must-pass

### F-007

**UAC-024**
- Given: モバイル gate true・閲覧モードでキーボード FAB が可視
- When: キーボード FAB 要素を観測する
- Then: `role=button` かつ `getBoundingClientRect().width≥44` かつ `height≥44` かつ非空の aria-label を持つ
- **Counterexample**: 32px アイコンボタン + aria-label 省略にすると width/height が 44 未満で aria-label が空のため、サイズ assertion と aria-label 非空 assertion で fail する (FR-A11Y-001 違反)。legacy には FAB 自体が無く計測対象が存在しないため必ず fail する。
- **vs Legacy**: must-fail

**UAC-025**
- Given: モバイル gate true・閲覧モードで terminal-host の `getBoundingClientRect` を T0 として記録済み
- When: キーボード FAB tap → ↓最新 FAB 出現まで状態を変化させる
- Then: terminal-host の `getBoundingClientRect` が T0 と一致し続ける (FAB 出現/状態変化で terminal box が縮まない=overlay 配置である)
- **Counterexample**: FAB を terminal-host の flex 兄弟として通常フローに挿入すると FAB 出現時に `flex:1 1 0` の terminal-host が残余を奪われ box が縮み height が変わるため、box 不変 assertion で fail する (ADR-0029/0065 違反)。
- **vs Legacy**: must-pass

**UAC-026**
- Given: モバイル gate true・入力モードでキーボード FAB が `aria-pressed="true"`
- When: 閲覧モードへ戻す (再 tap or blur)
- Then: 同じ FAB の `aria-pressed` が `"false"` に同期し aria-label が `"キーボードを開く"` に戻る
- **Counterexample**: aria-pressed を初期 render 時しか設定せず state 変化に同期しないと入力→閲覧後も `"true"` が残り、screen reader が誤った状態を読むため false 同期 assertion で fail する。
- **vs Legacy**: irrelevant

## Edge Cases

- **alt-screen 全画面プログラム (vim/less) 中**: ADR-0066 で scrollback が空のため ↓最新 FAB は出ない (scrollTop=末尾固定)。閲覧 swipe も移動量 0。alt-screen 終了後は scrollback 復活で FAB/swipe が機能する。
- **セッション未選択 (sessionId=null)**: host は出るが入力経路 drop。入力モード FAB tap で focus しても term.onData が sid 無しで drop (legacy 踏襲)。FAB は表示してよいが no-op。
- **iPadOS Safari が pointer:fine を報告**: gate false で desktop 扱い。幅 AND coarse の AND 条件で意図通り (外付けキーボード/トラックパッド iPad は desktop UX)。
- **デバイス回転で 767px 境界をまたぐ**: matchMedia の change を購読し mode を再評価。入力モード中に desktop へ移ったら入力モード state を破棄し readonly 解除。
- **iOS soft keyboard が dvh/layout viewport を縮めない**: ResizeObserver は発火しないため、visualViewport の resize/scroll を購読し FAB 群 bottom を `visualViewport.height + offsetTop` 基準で持ち上げ、入力行と退出 FAB をキーボード上端より上に保つ。**この経路は Playwright で実キーボードが立たないため実機 iOS Safari での手動検証が必要 (Open Questions)**。
- **入力モード遷移時の iOS focus-zoom**: `.xterm-helper-textarea` の font-size をグリッドの term.options.fontSize と独立に常時 16px 以上へ固定し、focus 時の意図しない viewport zoom を防ぐ (表示は `.xterm-rows` 側で別系統、ピンチ grid fontSize は 8–28px clamp 維持)。
- **ピンチ中の theme 変更**: useXtermTheme (ADR-0059) は options.theme のみ更新し fontSize に触れないため衝突しない。fontSize 永続値は theme と独立キー。
- **ピンチ直後の大量 output**: scheduleFit の rafPending ガード (ADR-0034) で fit() が単一 rAF に coalesce され refit と write が競合しない。
- **末尾判定の sub-pixel チラつき**: scrollTop が `scrollHeight-clientHeight` に端数で一致しない場合の FAB チラつきを防ぐため ±2px 近接を末尾扱いとする。
- **新規 animation の reduced-motion**: ↓最新 smooth scroll / FAB fade は ADR-0064 の view.css 末尾 `@media (prefers-reduced-motion: reduce)` 単一 guard ブロックに追記し、reduce では即時化する (集約先を分散させない)。
- **FAB の safe-area 二重計上回避**: `.app-shell` が四辺に env(safe-area-inset-*) を既適用 (FR-LAYOUT-004) のため、FAB は env(safe-area-inset) を再加算せず親 inset 内 16px offset で置く。
- **複数 FAB / toast の衝突**: 固定スタック (下:キーボード FAB → 上:↓最新 FAB, 8px gap)、font-size 制御は別位置、既存 `.notification-toast` と z-index/位置を分離し誤タップ/不可視を防ぐ。
- **localStorage 無効/容量超過 (private mode)**: 永続化失敗でも default 14px で動作継続 (try/catch、UX は degrade のみ)。
- **localStorage の不正/範囲外 fontSize**: 復元時に `[8,28]` clamp 検証、parse 不能/範囲外は default 14px フォールバック。
- **1 指→2 指の pinch 開始**: touchstart の touches.length 遷移で 1 指 swipe (scroll) と 2 指 pinch を分岐し誤判定でモード遷移しない。
- **coachmark 抑止**: 初回閲覧モード突入のみ表示、tap/数秒で dismiss、localStorage `web.term.hintSeen` で 2 回目以降は出さない。

## Open Questions

- **long-press 文字選択の標準 API 成立可否**: touch-action:pan-y 下で long-press 起点の xterm 文字選択が標準 API のみで成立するか、plan-how で addon 追加やカスタム selection handler が必要か (依存追加せず標準 API 優先の方針との整合)。F-003 の観察契約 (long-press→選択, swipe→scroll) は固定だが実現手段は plan-how が確定する。
- **iOS soft keyboard の visualViewport 連動の harness 限界**: Playwright (宣言した ATDD harness) は実 soft keyboard を表示せず visualViewport が縮まないため、入力モード中の FAB-lift と入力行可視は自動テストで判別できない。実機 iOS Safari での手動検証ステップを spec/plan 側で別途用意する必要がある。
- **font-size 制御の正確な配置と pinch との優先関係**: ＋/－/リセットを常時表示するか disclosure (アイコン → popover) にするか、視覚クラッタとの兼ね合い。SR 到達可能性 (role=button/44px/aria-label) は固定、視覚配置は plan-how/デザイン側で確定。
- **coachmark の表示時間/dismiss タイミング**: 「数秒で自動 dismiss」か「初回 tap で dismiss」か、または両方かの最終確定。localStorage 抑止キー (`web.term.hintSeen`) は固定。
- **F-006 の vs_legacy が全 must-pass である点の確認**: F-006 は PC 挙動の保護フローであり、vs_legacy semantics 上すべて `must-pass` (legacy 挙動を意図的に維持) が正しい。§8 の「各 flow に must-fail 1 件」は新規/変更フローの現状追認を排除する規則であり、保護フローはその例外とする (他の全フロー F-001〜F-005, F-007 は must-fail を 1 件以上保持)。この扱いをレビューで承認するか。
