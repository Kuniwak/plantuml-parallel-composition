# promotion 宣言と `csdfpromote`

改訂 2。改訂 1 からの主な変更：宣言を PlantUML ネイティブ構文（複合状態・`!include`・`note`）で綴る（§2）、上位互換パーサを `csdf/promote` に置き core は変えない（§3・§6）、展開先は親の複合状態で表す（§4.7）、局所 start edge の post は無視する（§4.2）、S₀ の自己ループは error（§4.6）、PNG の不変条件の扱い（§4.9）。

## 0. 背景と目的

CSDF は状態と状態変数、`event ; guard ; post` のエッジで LTS を書く。述語は自然言語（不透明）である。

同種のインスタンスが何個も並ぶシステム（口座 × N、約定 × N）を 1 本の CSDF で書くと、局所の制御状態はインスタンス ID → 局所状態 の写像（状態変数）に吸われ、大局図は「1 状態 + 自己ループ多数」になる。これは CSP の `||| id : ID @ Local(id)` を、状態を写像で持つ 1 本のパラメータ付き再帰方程式として書き直したもので、Z の promotion に一致する。

この形は検査（`csdfrefinement` / `csdflivelockfree` / `csdfrepl`）には適するが、人が読めない。読める形は「1 インスタンス分の局所図」である。両者は同じ成果物になりえないので、**局所図と結合宣言を人が書き、大局図（展開形）を機械生成する**。

生成の向きをこうする理由：

- 局所図は既に構造化された CSDF なので、大局への展開に新たな構造が要らない。逆向き（大局から局所を射影）は不透明述語の中に写像名・状態名の規約を入れる必要があり、CSDF の「述語は不透明」という設計と衝突する。
- 結合（写像を跨ぐガード、共有イベントの同時更新）は仕様の本体で、これを書く場所がないと「独立な写像の直積」という空の大局図ができる（実例あり）。結合は少数の宣言で書ける。

宣言は PlantUML のネイティブ構文で綴る。**手書きの大局図そのものが PlantUML でレンダリングでき、絵から promotion の構造が読める**ことを要件にする。展開形は道具用であり、絵にしない。

既存ツールは一切変更しない。`csdfhide` と同じ source-to-source の位置づけで新コマンド `csdfpromote` を足す。

### 語彙

`promote` は Z の語彙をそのまま使う。CSP で言い切れる部分は文書で対応を明記する：

| 宣言 | Z | CSP |
|---|---|---|
| `promote` | 自由 promotion（枠付け `θLocal = m(id)`, `m' = m ⊕ {id ↦ θLocal'}`） | `||| id : dom m @ Local(m(id))[[e ← e(id)]]` の記号形 |
| `sync` | 2 つの枠付けの連言 | アルファベット化並列 `[| {e} |]` |
| `constrain` | 制約付き promotion | 対応物なし（全状態を読む制約プロセスは大局そのもの） |

## 1. スコープ

### やること

1. 大局図に書く 3 種の宣言（`promote` / `sync` / `constrain`）を PlantUML ネイティブ構文で定義する。局所図の文法は変えない。
2. `csdf/promote` に CSDF の**上位互換パーサ**を置き、宣言を AST として取り出す。core の `csdf` パッケージの文法は変えない。
3. 新コマンド `csdfpromote`：宣言を含む大局図を読み、宣言をすべて消費した素の CSDF（展開形）を出力する。
4. core の `csdf` パーサは、宣言の痕跡（`<<promote>>`・`!include`・`note`）を見つけたら「`csdfpromote` を先に実行せよ」というヒント付きエラーで落とす。他ツールの挙動はこれで自動的に決まる。
5. lint（構造検査）を `csdfpromote` に内蔵する。
6. ドキュメント（SYNTAX.md / README.md / GLOSSARY.md / 新規 docs/PROMOTION.md）、`csdfhelp` への登録、golden test、PlantUML レンダリングのスモークテスト。

### やらないこと（Phase 2 以降）

- 列挙形（有限 ID での直積展開）と `csdfrename`。記号形との相互検算に使うが、まず記号形を出す。
- 大局図から局所図への射影（`csdfproject`）。局所図が正本なので不要。
- `csdfcomp` の composition tree への `PROMOTE` ノード追加。宣言は状態変数を参照するので、状態変数を持たない tree には載せない。
- 一般の階層状態。複合状態は `<<promote>>` ブロックとしてだけ受け付ける。

## 2. 宣言の綴り方（大局図）

### 2.1 例

```plantuml
@startuml CUSTODY-SPEC-STATE
!define PROMOTED

state "稼働中" as running {
  running : whitelistEntries
  running : buyTrades
  running : segReportCycles
  running : sessions

  state "whitelistEntries : エントリID ⇸ Whitelist" as whitelistEntries <<promote>> {
    !include states/WHITELIST_ja.puml
  }
  state "buyTrades : 買い約定ID ⇸ BuyTrade" as buyTrades <<promote>> {
    !include states/BUY_ja.puml
  }
  state "segReportCycles : 基準日 ⇸ SegReport" as segReportCycles <<promote>> {
    !include states/SEGREPORT_ja.puml
  }
  state "sessions : ユーザー ⇸ Session" as sessions <<promote>> {
    !include states/SESSION_ja.puml
  }
}

[*] --> running : すべての写像は空

note as sync1
  sync EVT-CUSTODY-DEPOSIT-UNKNOWN-BOOK : buyTrades(買い約定ID), segReportCycles(基準日)
end note
sync1 .. buyTrades
sync1 .. segReportCycles

note as cons1
  constrain EVT-HW-WHITELIST-VERIFY(売り約定ID, 送金先アドレス) ; whitelistEntries の 送金先アドレス が〈登録済み〉
end note
cons1 .. whitelistEntries

note as cons2
  constrain EVT-WL-CHECKER-APPROVE(エントリID, checker) ; sessions の checker が〈ログイン中〉で役割が checker
end note
cons2 .. sessions
cons2 .. whitelistEntries
@enduml
```

レンダリング結果：「稼働中」の箱の中に家族ごとの箱（見出し `写像 : ID ⇸ 型`）が並び、中身は局所のライフサイクル、共有イベントと制約は該当する箱に線で繋がった注記になる。「箱 1 つ = ID ごとに 1 部」という読みが絵から取れる。

### 2.2 宣言と PlantUML 構文の対応

| 宣言 | PlantUML の綴り | 抽出する情報 |
|---|---|---|
| `promote` | `state "<map> : <ID> ⇸ <Type>" as <map> <<promote>> { !include <path> }` | 別名 `<map>`＝写像名。見出しから `<ID>`（インスタンス ID のパラメータ名）と `<Type>`。`!include` からパス |
| `sync` | 先頭行が `sync` で始まる `note as <alias> … end note` | 本文を `syncBody` 文法で解釈 |
| `constrain` | 先頭行が `constrain` で始まる `note as <alias> … end note` | 本文を `constrainBody` 文法で解釈 |
| 展開先 | `<<promote>>` 状態を置いた**親の複合状態** | 複数の親に置きたければ、同じ写像の `<<promote>>` ブロックを各親に書く |

`note .. state` の接続線は絵のためだけで、意味は本文から取る（接続先と本文の不一致は lint warning、§4.6）。先頭行が `sync`/`constrain` でない `note` は無視する（描画専用）。

### 2.3 ABNF（`csdf/promote` の上位互換文法）

core の `diagram` に対し、`stateDecl` の位置に `compositeState`、`edgeDecl` の位置に `noteBlock` / `noteLink` / `preprocessorLine` を許す。

```
globalDiagram   = "@startuml" inlineTrivia 0*1(diagramName) LF trivia
                  *(preprocessorLine trivia)
                  1*((stateDecl / compositeState) trivia)
                  startEdgeDecl trivia
                  *((edgeDecl / noteBlock / noteLink) trivia)
                  0*1(endEdgeDecl trivia)
                  "@enduml" LF

compositeState  = "state" inlineSeparator quotedName inlineSeparator "as" inlineSeparator stateID
                  [inlineSeparator stereotype] inlineTrivia "{" LF trivia
                  *((stateVarDecl / promoteBlock) trivia)
                  "}" LF
                  ; stereotype なし：単一実体の状態（複数状態の大局図で使う）。中身は stateVarDecl と promoteBlock のみ。
                  ; ネストは promoteBlock のみ許す。一般の階層状態は error。

promoteBlock    = "state" inlineSeparator promoteTitle inlineSeparator "as" inlineSeparator stateID
                  inlineSeparator "<<promote>>" inlineTrivia "{" LF trivia
                  includeLine trivia
                  "}" LF
promoteTitle    = DQUOTE var inlineTrivia ":" inlineTrivia param inlineTrivia "⇸" inlineTrivia typeName DQUOTE
                  ; stateID は var と一致しなければならない（lint error）
includeLine     = "!include" inlineSeparator path inlineTrivia LF

noteBlock       = "note" inlineSeparator "as" inlineSeparator noteID inlineTrivia LF
                  *(noteLine LF)
                  inlineTrivia "end note" inlineTrivia LF
noteLine        = *unicode_char_except_LF
                  ; 最初の非空行が "sync" / "constrain" で始まる noteBlock だけを宣言として解釈する
noteLink        = noteID inlineTrivia ".." inlineTrivia stateID inlineTrivia LF
                / stateID inlineTrivia ".." inlineTrivia noteID inlineTrivia LF

syncBody        = "sync" inlineSeparator eventName inlineTrivia ":" inlineTrivia mapRef *(inlineTrivia "," inlineTrivia mapRef)
constrainBody   = "constrain" inlineSeparator eventPattern inlineTrivia ";" inlineTrivia guard
mapRef          = var inlineTrivia "(" inlineTrivia param inlineTrivia ")"
eventPattern    = eventName inlineTrivia "(" inlineTrivia param *(inlineTrivia "," inlineTrivia param) inlineTrivia ")"

preprocessorLine = "!" 1*unicode_char_except_LF LF      ; !define / !ifndef など。読み飛ばす
path            = 1*(unicode_char_except_space) / DQUOTE 1*(unicode_char_except_DQUOTE) DQUOTE
typeName        = id
param           = 1*(unicode_char_except_comma_paren_semicolon)
eventName       = 1*(unicode_char_except_paren_semicolon)
```

`⇸` は U+21F8。ASCII 代替として `->>` も受理する（docs に明記）。

### 2.4 局所図側の約束

局所図の**文法は変えない**が、`!include` されることを前提に次を守る（lint と docs で強制）：

- `title` / `note` / `skinparam` など PlantUML 専用行は `!ifndef PROMOTED … !endif` で包む（大局図の先頭で `!define PROMOTED` する）。CSDF-IGNORE 領域と重なるのでそこに入れるだけでよい。
- 状態 ID（`as` の別名）は局所図間で一意にする。接頭辞（`wl`, `buy`, …）を推奨。
- start 状態 S₀ は状態変数を持たない。S₀ を終点・始点とするエッジ、および局所 start edge に post を書かない。end edge を置かない（§4.1）。

## 3. 型（AST）

`csdf/promote` パッケージ内。core の `csdf.Diagram` は変更しない。

```go
type GlobalDiagram struct {
    Core       csdf.Diagram   // 宣言を除いた大局図（複合状態は平坦化して通常の状態にする）
    Promotes   []Promote
    Syncs      []Sync
    Constrains []Constrain
    Notes      []NoteLink     // 描画用。lint の突合にだけ使う
}

type Promote struct {
    Map     csdf.Var
    IDParam string
    Type    string
    Path    string
    In      csdf.StateID    // 親の複合状態
}

type Sync struct {
    NoteID  string
    Event   string          // 局所イベント名（括弧の前）
    Targets []MapRef
}

type MapRef struct {
    Map   csdf.Var
    Param string
}

type Constrain struct {
    NoteID string
    Event  string          // 昇格後イベント名
    Params []string
    Guard  string
}

type NoteLink struct {
    NoteID string
    State  csdf.StateID
}
```

`csdfparse` の JSON は変更しない（core の図しか扱わない）。`csdfpromote -json` で `GlobalDiagram` を JSON で出せるようにする（デバッグ用、任意）。

## 4. 展開の意味論

以下、宣言 `promote` の親状態を `G`、写像を `m`、局所図を `L`、局所図の start edge の行き先を `S₀`、局所イベントを `e(a₁, …, aₙ)`（引数なしなら `e`）とする。

### 4.1 前提（局所図の規約、lint で強制）

- `S₀` は「そのインスタンスが存在しない」を意味する。したがって
  - `S₀` は状態変数を持たない（error）
  - `S₀` から出るエッジ＝インスタンスの生成（`m` にキーを追加）
  - `S₀` へ入るエッジ＝インスタンスの削除（`m` からキーを除く）
  - `S₀ --> S₀` の自己ループは「存在しないインスタンスに対するイベント」で意味を持たない（error）
- end edge（`--> [*]`）は禁止（error）。完了は後続遷移を持たない状態として書く。
- `tau` は許す（§4.3）。

### 4.2 通常のエッジ

局所 `S --> T : e(a₁,…,aₙ) ; g ; p` は次の 1 本に展開する（`id` は `<IDParam>`）：

```
G --> G : e(id, a₁, …, aₙ) ; id ∈ dom m ∧ m(id) ∈ 〈S〉 ∧ g ; m' = m ⊕ {id ↦ 〈T〉(…)} ∧ p ∧ FRAME(m)
```

- **生成**（`S = S₀`）：ガードは `id ∉ dom m ∧ g`、事後条件は `m' = m ∪ {id ↦ 〈T〉(…)} ∧ p ∧ FRAME(m)`。`T` の変数の初期値は **この `p` がすべて決める**。
- **削除**（`T = S₀`）：ガードは通常どおり、事後条件は `m' = {id} ⩤ m ∧ FRAME(m)`。`p` は使わない（§4.6 の warning）。
- **局所 start edge の post は使わない。** S₀ は変数を持たないので、その付値についての述語は無意味である。start edge に post があれば warning を出して無視する（改訂 1 の「生成エッジに連言する」は撤回）。
- `FRAME(m)`：「`m` の他のキーは不変、他の写像は不変、`G` の写像以外の変数は不変」。
- `g` / `p` は不透明なので**字面のまま**埋め込む。局所変数 `v` は昇格後 `m(id).v` を指すが、置換はしない（述語は不透明であり、直前の由来コメント §4.8 で文脈が判る）。
- 同じ `(S, e)` から複数のエッジ（非決定・ガード分岐）はそのまま複数本になる。
- 生成エッジが複数あって行き先が異なる場合、各エッジの `p` が各行き先の初期値を決める。共通の初期値は各エッジに書く（重複は無害、片方だけの初期値が黙って両方に付く事故を避ける）。

### 4.3 `tau`

局所の `tau` エッジは `tau` のまま昇格する（インスタンス ID は付けない。付けると観測可能になり隠蔽の意味が変わる）。ガード・事後条件は §4.2 と同じで、`id` は暗黙に存在量化される（「ある `id` について…」）。

昇格後の `tau` は自己ループだが、事後条件で `m(id)` の局所状態が変わるので、局所図に `tau` サイクルがなければ発散しない（証明義務：「〈S〉にある実例の数が減る」）。`csdflivelockfree` は「構造的には livelock-free と言えない」と報告し、述語付きの義務を出す。これは正しい挙動であり、`tau` を禁止する理由にはならない（docs に書く）。

### 4.4 `sync`

`sync e : m₁(p₁), m₂(p₂)` があるとき、局所 L₁ の `e` エッジ群 E₁ と L₂ の `e` エッジ群 E₂ の**直積**の各組 `(x₁, x₂)` について 1 本に併合する：

```
G --> G : e(p₁, p₂, args…) ; GUARD(x₁)[id:=p₁] ∧ GUARD(x₂)[id:=p₂] ; POST(x₁)[id:=p₁] ∧ POST(x₂)[id:=p₂] ∧ FRAME(m₁, m₂)
```

- 引数：各局所のイベント引数を順に連結し、同名の引数は 1 つにまとめる（名前一致で判定。同名で意味が違う引数は誤結合するので docs に注意書き）。
- `sync` に挙げられた写像の `e` エッジは単独では展開しない（併合したものだけを出す）。
- 挙げられていない写像に同名イベントがあれば独立に展開する（かつ warning、§4.6）。
- 展開先は対象写像の展開先集合（§4.7）の**交わり**。空なら error。

### 4.5 `constrain`

`constrain e(q₁,…,qₖ) ; c` があるとき、展開後のイベント名 `e` で引数数 `k` の全エッジ（生成・削除・`sync` 済みを含む、すべての展開先状態）のガードに `∧ c` を連言する。`c` は不透明で置換しない。`c` は展開後の引数名で書かれていることを期待し、`q₁…qₖ` のいずれも `c` に現れなければ warning。

### 4.6 lint

| 種別 | 内容 |
|---|---|
| error | `<<promote>>` ブロックの `as` 別名が見出しの写像名と一致しない |
| error | `<<promote>>` ブロックの写像が親状態の状態変数にない |
| error | `<<promote>>` ブロックの中身が `!include` 1 行でない |
| error | `!include` のパスが解決できない / 局所図が CSDF として読めない |
| error | `<<promote>>` 以外のネストした複合状態 |
| error | 局所図の `S₀` が状態変数を持つ |
| error | 局所図に end edge がある |
| error | 局所図に `S₀ --> S₀` のエッジがある |
| error | 局所図間で状態 ID が重複する（`!include` で PlantUML が同一視する） |
| error | `sync` のイベントがいずれかの対象局所図に存在しない |
| error | `sync` の対象写像が `promote` されていない |
| error | `sync` の対象写像の展開先の交わりが空 |
| error | `constrain` のイベント名・引数数に一致する展開後エッジがない |
| error | 同じ親状態で同じ写像を 2 回 `promote` している |
| error | `sync`/`constrain` の note 本文が `syncBody`/`constrainBody` として読めない |
| warning | 局所 start edge に post がある（無視される。省略せよ） |
| warning | 削除エッジ（`T = S₀`）に post がある（捨てられる。他写像への効果なら `sync` か大局の手書きエッジで） |
| warning | 同じ局所イベント名が 2 つ以上の局所図にあり `sync` されていない（共有イベントの見落とし） |
| warning | `sync`/`constrain` の note の接続先（`..`）が本文に現れる写像と一致しない |
| warning | `constrain` のガード文に引数名が 1 つも現れない |
| warning | `<<promote>>` の型名が親状態の該当 `varType` 文字列に現れない |
| info | 写像を状態変数に持つが `<<promote>>` ブロックを置いていない状態がある（その状態では家族が凍結） |

warning は stderr、error は exit 1。`-Werror` で warning も exit 1。

### 4.7 展開先（複数状態の大局図）

- `<<promote>>` ブロックを置いた親の複合状態がその家族の展開先。省略記法はない（1 状態の大局図でも親を書く。§2.1 の形）。
- 同じ写像を複数の親に置けば、各親に自己ループ群が出る。写像を状態変数に持つが `<<promote>>` を置かない状態では家族は凍結する（意図的な表現。info）。
- 親状態間の遷移（モード切替 `running --> maintenance : …`）は手書き。フレーム「すべての写像は不変」も手書き。promotion の外側なので生成しない。
- **自動展開はしない**：写像を持つ全状態へ暗黙に展開すると、メンテナンス中でも約定が進む仕様が黙って生まれる。

### 4.8 出力（展開形）

- 素の CSDF。宣言・`!include`・`note`・preprocessor 行は消える。複合状態は通常の状態に平坦化する（`state "稼働中" as running` と `running : var` 行）。
- 手書きの状態・start edge・エッジはそのまま残す。
- 展開エッジは `csdfsort` と同じ正規順序（source, event, destination, guard, post）で並べる。手書きエッジも同じ順序に混ぜる。
- 展開エッジの直前に由来を示す line comment を 1 行出す（`' promote: WHITELIST_ja.puml 〈未登録〉 → 〈CBJ登録承認待ち通知前〉`、`sync` は `' sync: EVT-… (BUY_ja.puml 〈…〉→〈…〉) × (SEGREPORT_ja.puml 〈…〉→〈…〉)`）。`-no-comments` で抑止。
- `@startuml auto-generated-by: csdfpromote <args>`（POSIX クォート、既存慣例）。
- 入力大局図の `CSDF-IGNORE` 領域はそのまま保持する。

### 4.9 レンダリングと PNG

- **手書きの大局図**は PlantUML でレンダリングできることを要件とする（§7 のスモークテスト）。
- **展開形**は絵にしない（1 状態 + 自己ループ多数で読めない）。利用側の「すべての `*.puml` に兄弟 PNG」のような不変条件は「手書きの `*.puml` に」へ緩めることを docs で勧める。

### 4.10 述語のテンプレート

生成する文言（`id ∈ dom m`, `m(id) ∈ 〈S〉`, `m' = m ⊕ {…}`, FRAME）は記号表記を既定にし、言語に依存させない。`-template <file>` で置換可能（Go `text/template`、フィールド：`Map`, `ID`, `Src`, `Dst`, `Guard`, `Post`, `OtherMaps`, `OtherVars`）。日本語版を `examples/promote/templates/ja.tmpl` として同梱。

## 5. CLI

```
csdfpromote [-base DIR] [-template FILE] [-no-comments] [-Werror] [-lint-only] [-json] <file|->
```

- 入力は `.puml` または PlantUML 生成の `.png`（既存と同じ）。stdin / `-` 等価。
- `-lint-only`：展開せず lint だけ実行。exit code で結果を返す。
- `-base`：`!include` のパス解決の基点（既定：入力ファイルのディレクトリ、stdin ならカレント）。
- `-json`：`GlobalDiagram` を JSON で出す（デバッグ用）。
- 出力は stdout。差分検査は利用側 CI で `diff` する（`-check` は持たない）。

`csdfhelp` に登録し、`README.md` に「Promotion」節を追加する。

## 6. パッケージ構成

```
csdf/                  既存。文法は変更しない。宣言の痕跡（"<<promote>>", "!include", "note") を見つけたときの
                       エラーメッセージに「run csdfpromote first」のヒントを足すだけ
csdf/promote/parser.go 上位互換パーサ（§2.3）。core の字句・エッジ解釈は csdf パッケージのものを再利用する
csdf/promote/expand.go 展開（§4.2〜4.5）。Expand(g GlobalDiagram, load func(path) (csdf.Diagram, error), opts) (csdf.Diagram, []Diagnostic, error)
csdf/promote/lint.go   §4.6
cli/csdfpromote/       CLI
docs/PROMOTION.md      意味論、CSP/Z 対応、tau の扱い、置換しない判断、start/削除エッジの post、sync の引数結合、
                       局所図側の約束（!ifndef PROMOTED、状態 ID の接頭辞）、PNG の扱い
examples/promote/      golden test の入力と期待出力、日本語テンプレート
```

core への変更は「ヒント付きエラー」だけに限定する。ヒントは `csdf` のパースエラーに文脈として付けるので、全 CLI に自動で効く。

## 7. テスト

1. **上位互換パーサ**：§2.3 の受理・拒否（`<<promote>>` の中身が複数行、一般の階層状態、`as` 別名と見出しの不一致、note 本文の構文エラー、preprocessor 行の読み飛ばし）。core の既存 golden test がすべて通ること（core は無変更なので通るはず。ヒント付きエラーのメッセージ変更分だけ期待値を更新）。
2. **展開 golden test**（`examples/promote/`）：
   - 単一局所・状態変数なし（`A → B → STOP`）
   - 状態変数あり、生成・削除・自己ループ・非決定分岐
   - 生成エッジが 2 本で行き先が異なる局所（各エッジの post だけが展開に現れる）
   - `tau` を含む局所
   - 2 局所 + `sync`（直積 1×1、2×1、2×2）
   - `constrain` が生成・通常・sync 済みエッジに当たる
   - 手書き大局エッジとの混在
   - 複数状態の大局図：同じ写像を 2 つの親に置く／片方にだけ置く（凍結）／モード切替の手書きエッジ
   - 日本語テンプレート
3. **lint テスト**：§4.6 の各行に 1 ケース。
4. **core の拒否**：宣言入りの大局図を各ツールに渡し、ヒント付きエラーを確認。
5. **PlantUML スモークテスト**：`examples/promote/` の各大局図を `plantuml -checkonly`（または実レンダリング）に通す。特に確認する点：
   - `!include` した局所図の `@startuml`/`@enduml` が剥がされて取り込まれる（剥がされなければ `!startsub`/`!includesub` に切り替える）
   - `!ifndef PROMOTED` で局所の `title`/`note` が消える
   - 複合状態ごとの `[*]` が描ける
   - `note .. state` の接続線が描ける
6. **`csdfrepl`**：展開形を 0-switch でアニメーションできる（写像の値は JSON オブジェクト）。
7. **列挙形との相互検算**（Phase 2）：有限 ID（2〜3 個）で `csdfrename` + `csdfparallel` の直積を作り、`csdfrefinement -m f` の義務を両方向に生成する。

## 8. マイルストーン

| # | 内容 | 受け入れ条件 |
|---|---|---|
| M0 | PlantUML スモークテスト（§7-5）を最初に実行する | `!include` の挙動が確定し、§2 の綴り方が固まる |
| M1 | 上位互換パーサ・AST・`-json`・core のヒント付きエラー | 既存 golden test 全通過。§2.1 の例が AST になる |
| M2 | `promote` のみの展開、lint の error 群、複数状態の大局図 | 単一局所と複数状態の golden test 通過。`csdflivelockfree` / `csdfrepl` に通る |
| M3 | `sync` / `constrain` / warning・info 群 / `-template` | 全 golden test 通過 |
| M4 | ドキュメント、`csdfhelp`、README、SYNTAX.md（上位互換文法の節）、GLOSSARY.md | `csdfhelp csdfpromote` が出る。ABNF が実装と一致 |
| M5（Phase 2） | `csdfrename`、列挙形、相互検算 | 有限 ID で両形が出て、義務ファイルが生成される |

## 9. 決定済み事項（改訂 1 の §9 を確定）

1. **イベント引数の記法**：`e(id, a, b)`。ID は第 1 引数。`e.id` は採らない。
2. **局所変数を置換しない**。述語は不透明で、字面置換は壊れうる。由来コメントで文脈を示す。
3. **`sync` の引数結合**：名前一致で重複を潰す。docs に注意書き。
4. **`tau` に ID を付けない**。
5. **削除エッジの post は warning を出して捨てる**。無警告にしたければ post を省略する（文法上省略可）。局所 start edge の post も同様に warning を出して無視する。
6. **展開先は親の複合状態で表す**。自動展開なし。写像を持つが `<<promote>>` を置かない状態は凍結（info）。
7. **宣言は PlantUML ネイティブ構文**。独自行は入れない。手書きの大局図はレンダリング可能であること。
8. **上位互換パーサは `csdf/promote` に置き、core は変えない**。

## 10. 未決事項（実装中に判断）

- `!include` が `@startuml` を剥がさない場合の代替（`!startsub`/`!includesub`）。M0 で確定する。
- `⇸` の ASCII 代替 `->>` を本当に受理するか（PlantUML の見出しに `->>` が出ても描画は問題ない。読みやすさの判断）。
- `note` の本文に複数の `sync`/`constrain` を書けるようにするか。M3 では 1 note 1 宣言に限定する。
