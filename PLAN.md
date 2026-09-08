# promotion 宣言と `csdfpromote`

## 0. 背景と目的

CSDF は状態と状態変数、`event ; guard ; post` のエッジで LTS を書く。述語は自然言語（不透明）である。

同種のインスタンスが何個も並ぶシステム（口座 × N、約定 × N）を 1 本の CSDF で書くと、局所の制御状態はインスタンス ID → 局所状態 の写像（状態変数）に吸われ、大局図は「1 状態 + 自己ループ多数」になる。これは CSP の `||| id : ID @ Local(id)` を、状態を写像で持つ 1 本のパラメータ付き再帰方程式として書き直したもので、Z の promotion に一致する。

この形は検査（`csdfrefinement` / `csdflivelockfree` / `csdfrepl`）には適するが、人が読めない。読める形は「1 インスタンス分の局所図」である。両者は同じ成果物になりえないので、**局所図と結合宣言を人が書き、大局図を機械生成する**。

生成の向きをこうする理由：

- 局所図は既に構造化された CSDF なので、大局への展開に新たな構造が要らない。逆向き（大局から局所を射影）は不透明述語の中に写像名・状態名の規約を入れる必要があり、CSDF の「述語は不透明」という設計と衝突する。
- 結合（写像を跨ぐガード、共有イベントの同時更新）は仕様の本体で、これを書く場所がないと「独立な写像の直積」という空の大局図ができる（実例あり）。結合は少数の宣言で書ける。

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

1. CSDF 文法に 3 種の宣言（`promote` / `sync` / `constrain`）を追加する。宣言は**大局図**に書く。局所図の文法は変えない。
2. `csdf` パッケージの AST に宣言を保持するフィールドを追加し、`csdfparse` の JSON に出す。
3. 新コマンド `csdfpromote`：宣言を含む図を読み、宣言をすべて消費した素の CSDF を出力する。
4. 宣言を含む図を他のツール（`csdfparallel`, `csdfhide`, `csdfnorm`, `csdfsort`, `csdflivelockfree`, `csdfrefinement`, `csdfrepl`, `csdfcomp`）に渡した場合は「`csdfpromote` を先に実行せよ」というエラーで落とす。
5. lint（構造検査）を `csdfpromote` に内蔵する。
6. ドキュメント（SYNTAX.md / README.md / GLOSSARY.md / 新規 docs/PROMOTION.md）、`csdfhelp` への登録、golden test。

### やらないこと（Phase 2 以降）

- 列挙形（有限 ID での直積展開）と `csdfrename`。記号形との相互検算に使うが、まず記号形を出す。
- 大局図から局所図への射影（`csdfproject`）。局所図が正本なので不要。
- `csdfcomp` の composition tree への `PROMOTE` ノード追加。宣言は状態変数を参照するので、状態変数を持たない tree には載せない。

## 2. 文法

SYNTAX.md の ABNF に追記する。`diagram` の `*(edgeDecl trivia)` の位置に宣言を混在させられるようにする（順序は自由）。

```
diagram      = "@startuml" inlineTrivia 0*1(diagramName) LF trivia 1*(stateDecl trivia) startEdgeDecl trivia *((edgeDecl / promoteDecl / syncDecl / constrainDecl) trivia) 0*1(endEdgeDecl trivia) "@enduml" LF

promoteDecl   = "promote" inlineSeparator path inlineSeparator "as" inlineSeparator typeName inlineSeparator "via" inlineSeparator mapRef 0*1(inlineSeparator inClause) inlineTrivia LF
inClause      = "in" inlineSeparator stateID *(inlineTrivia "," inlineTrivia stateID)
syncDecl      = "sync" inlineSeparator eventName inlineTrivia ":" inlineTrivia mapRef *(inlineTrivia "," inlineTrivia mapRef) inlineTrivia LF
constrainDecl = "constrain" inlineSeparator eventPattern inlineTrivia ";" inlineTrivia guard inlineTrivia LF

mapRef        = var inlineTrivia "(" inlineTrivia param inlineTrivia ")"
eventPattern  = eventName inlineTrivia "(" inlineTrivia param *(inlineTrivia "," inlineTrivia param) inlineTrivia ")"
path          = 1*(unicode_char_except_space)      ; ダブルクォートで囲めば空白を含めてよい
typeName      = id
param         = 1*(unicode_char_except_comma_paren_semicolon)
eventName     = 1*(unicode_char_except_paren_semicolon)
```

- `promote <path> as <Type> via <map>(<idParam>) [in <stateID>, …]`
  - `<path>` は大局図のファイルからの相対パス。stdin のときはカレントディレクトリ。`-base` で上書き（`csdfcomp` と同じ規則）。
  - `<Type>` は局所図の状態型に付ける名前。`<map>` の `varType` に現れることを期待するが、`varType` は自由文であり検査しない（警告のみ）。
  - `<map>` は `in` に挙げた各状態が持つ状態変数でなければならない（lint）。
  - `<idParam>` はインスタンス ID のパラメータ名。昇格後のイベントの第 1 引数になる。
  - `in` は展開先の大局状態の集合。**省略時は `startEdgeDecl.dst` の 1 状態**。挙げた各状態に同じ展開を複製する。運転モード（`running` / `degraded` 等）ごとに同じ写像を動かしたいときに複数書く。

- `sync <event> : <map1>(<p1>), <map2>(<p2>), …`
  - `<event>` の局所イベント名（括弧の前まで）。指定した各写像の局所図にそのイベントが存在しなければならない。
  - 展開先の状態は、**対象写像それぞれの `in` 集合の交わり**。交わりが空なら lint error（どの大局状態でも同時に起きえない事象を同期しようとしている）。

- `constrain <event>(<params>) ; <guard>`
  - `<event>(<params>)` は昇格後のイベント（第 1 引数がインスタンス ID）。パラメータ数は昇格後のイベントと一致しなければならない。
  - `<guard>` は不透明述語。展開時に該当エッジのガードへ連言する。
  - **大局状態には依存しない**。イベント名と引数数が一致する展開後エッジすべてに当たる（複数の `in` 状態に複製されたエッジにも当たる）。

モード切替のエッジ（`running --> maintenance : …`）とそのフレーム（「すべての写像は不変」）は**手書き**である。promotion の外側なので `csdfpromote` は生成しない。

PlantUML はこれらの行を解釈できないので、レンダリングには `csdfpromote` の出力を使う。生成側の `@startuml` 名は `auto-generated-by: csdfpromote …` にする（既存慣例）。

## 3. 型（AST）

```go
type Diagram struct {
    Name       string
    States     map[StateID]State
    StartEdge  StartEdge
    Edges      []Edge
    EndEdge    *EndEdge
    Promotes   []Promote     // 追加
    Syncs      []Sync        // 追加
    Constrains []Constrain   // 追加
}

type Promote struct {
    Path    string
    Type    string
    Map     Var
    IDParam string
    In      []StateID  // 空なら StartEdge.Dst の 1 状態
}

type Sync struct {
    Event   string   // 局所イベント名（括弧の前）
    Targets []MapRef
}

type MapRef struct {
    Map   Var
    Param string
}

type Constrain struct {
    Event  string   // 昇格後イベント名
    Params []string
    Guard  string
}
```

`csdfparse` の JSON では `promotes` / `syncs` / `constrains` を追加（空なら `[]`）。既存のキーは変えない。

`Diagram` に `HasDirectives() bool` を足し、`csdfpromote` 以外のツールは読み込み直後にこれを見て

```
diagram contains promotion directives (promote/sync/constrain); run csdfpromote first
```

で exit 1 する。

## 4. 展開の意味論

以下、展開先の大局状態を `G`（`in` に挙げた各状態。省略時は start 状態）、写像を `m`、局所図を `L`、局所図の start edge の行き先を `S₀`、局所イベントを `e(a₁, …, aₙ)`（引数なしなら `e`）とする。

### 4.1 前提（局所図の規約、lint で強制）

- `S₀` は「そのインスタンスが存在しない」を意味する。したがって
  - `S₀` は状態変数を持たない（持っていたら lint error）
  - `S₀` から出るエッジ = インスタンスの生成（`m` にキーを追加）
  - `S₀` へ入るエッジ = インスタンスの削除（`m` からキーを除く）
- end edge（`--> [*]`）は禁止（lint error）。完了は後続遷移を持たない状態として書く。
- `tau` は許す（下記 4.3）。

### 4.2 通常のエッジ

局所 `S --> T : e(a₁,…,aₙ) ; g ; p` は、`in` の各状態 `G` について次の 1 本に展開する（`id` は `<idParam>`）：

```
G --> G : e(id, a₁, …, aₙ) ; id ∈ dom m ∧ m(id) ∈ 〈S〉 ∧ g ; m' = m ⊕ {id ↦ 〈T〉(…)} ∧ p ∧ FRAME(m)
```

- 生成（`S = S₀`）：ガードは `id ∉ dom m ∧ g`、事後条件は `m' = m ∪ {id ↦ 〈T〉(…)} ∧ p ∧ FRAME(m)`。
- 削除（`T = S₀`）：ガードは通常どおり、事後条件は `m' = {id} ⩤ m ∧ FRAME(m)`（`p` は捨てる。`S₀` は変数を持たないので捨ててよい。`p` が空でなければ warning §4.6）。
- `FRAME(m)`：「`m` の他のキーは不変、他の写像は不変」。
- 局所の start edge の `post`（初期値の述語）は、生成エッジの `p` に連言する。
- `g` / `p` は不透明なので**字面のまま**埋め込む。局所変数 `v` は昇格後 `m(id).v` を指すが、置換はしない（述語は不透明であり、字面置換で壊れうる。読者は直前の由来コメントで文脈が判る）。この判断は docs/PROMOTION.md に明記する。
- 同じ `(S, e)` から複数のエッジ（非決定・ガード分岐）はそのまま複数本になる。

### 4.3 `tau`

局所の `tau` エッジは `tau` のまま昇格する（インスタンス ID は付けない。付ければ観測可能になり、隠蔽の意味が変わる）。ガード・事後条件は 4.2 と同じで、`id` は暗黙に存在量化される（「ある `id` について…」）。

昇格後の `tau` は自己ループだが、事後条件で `m(id)` の局所状態が変わるので、局所図に `tau` サイクルがなければ発散しない（証明義務：「〈S〉にある実例の数が減る」）。`csdflivelockfree` は「構造的には livelock-free と言えない」と報告し、述語付きの義務を出す。これは正しい挙動であり、`tau` を禁止する理由にはならない。docs に書く。

### 4.4 `sync`

`sync e : m₁(p₁), m₂(p₂)` があるとき、局所 L₁ の `e` エッジ群 E₁ と L₂ の `e` エッジ群 E₂ の**直積**の各組 `(x₁, x₂)` について、`m₁` と `m₂` の `in` 集合の**交わり**の各状態 `G` に 1 本を出す：

```
G --> G : e(p₁, p₂, args…) ; GUARD(x₁)[id:=p₁] ∧ GUARD(x₂)[id:=p₂] ; POST(x₁)[id:=p₁] ∧ POST(x₂)[id:=p₂] ∧ FRAME(m₁, m₂)
```

- 引数：各局所のイベント引数を順に連結し、重複する引数名は 1 つにまとめる（名前の一致で判定。名前が同じで意味が違う引数があると誤結合するので、docs に注意書き）。
- `sync` に挙げられた写像の `e` エッジは、単独では展開しない（併合したものだけを出す）。
- 挙げられていない写像に同名イベントがあれば、それは独立に展開する（かつ warning、4.6）。

### 4.5 `constrain`

`constrain e(q₁,…,qₖ) ; c` があるとき、展開後のイベント名 `e` で引数数 `k` の全エッジのガードに `∧ c` を連言する。大局状態には依存しない。引数名は位置対応で `q_i` を展開後の引数名に置換して `c` に反映…**しない**（`c` は不透明）。代わりに、`c` は展開後の引数名で書かれていることを lint で要求する（`c` の中に `q₁…qₖ` のいずれも現れなければ warning）。

`constrain` は生成エッジ・削除エッジ・`sync` 済みエッジにも適用される。

### 4.6 lint

| 種別 | 内容 |
|---|---|
| error | `promote` の `<map>` が `in` に挙げた状態（省略時は start 状態）の状態変数にない |
| error | `promote` の `in` に挙げた状態が大局図に存在しない |
| error | `promote` の局所図が読めない / パスが解決できない |
| error | 局所図の `S₀` が状態変数を持つ |
| error | 局所図に end edge がある |
| error | `sync` のイベントがいずれかの対象局所図に存在しない |
| error | `sync` の対象写像が `promote` されていない |
| error | `sync` の対象写像の `in` 集合の交わりが空 |
| error | `constrain` のイベント名・引数数に一致する展開後エッジがない |
| error | 同じ `<map>` を 2 回 `promote` している |
| warning | 同じ局所イベント名が 2 つ以上の局所図にあり `sync` されていない（共有イベントの見落とし検出） |
| warning | `promote` の `<Type>` が `<map>` の `varType` 文字列に現れない |
| warning | `constrain` のガード文に引数名が 1 つも現れない |
| warning | 削除エッジ（`T = S₀`）の局所 `post` が空でない（捨てられる） |
| info | `<map>` を状態変数に持つが `in` に挙げられていない大局状態がある（その状態では写像が凍結される。読み上げるだけ） |

info は stderr に出すが exit code に影響しない。warning は stderr、error は exit 1。`-Werror` で warning も exit 1（info は対象外）。

### 4.7 出力

- 素の CSDF。宣言は消える。状態・start edge・大局図に手書きされたエッジはそのまま残す。
- 展開エッジは `csdfsort` と同じ正規順序（source, event, destination, guard, post）で並べる。手書きエッジも同じ順序に混ぜる。
- 展開エッジの直前に、由来を示す line comment を 1 行出す（`' promote: WHITELIST.puml 〈未登録〉 → 〈CBJ登録承認待ち通知前〉`）。CSDF は無視し、人が読める。`-no-comments` で抑止。
- `@startuml auto-generated-by: csdfpromote <args>`（POSIX クォート、既存慣例）。
- `CSDF-IGNORE` 領域と `title` 等の PlantUML 専用行は入力大局図のものを保持する。

### 4.8 述語のテンプレート

生成する自然言語部分（`id ∈ dom m`, `m(id) ∈ 〈S〉`, `m' = m ⊕ {…}`, FRAME）は記号表記を既定にし、言語に依存させない。`-template <file>` で置き換え可能にする（Go `text/template`、フィールド：`Map`, `ID`, `Src`, `Dst`, `Guard`, `Post`, `OtherMaps`）。日本語版テンプレートを `examples/promote/templates/ja.tmpl` として同梱する。

## 5. CLI

```
csdfpromote [-base DIR] [-template FILE] [-no-comments] [-Werror] [-lint-only] <file|->
```

- 入力は `.puml` または PlantUML 生成の `.png`（既存と同じ）。stdin / `-` 等価。
- `-lint-only`：展開せず lint だけ実行。exit code で結果を返す。
- `-base`：局所図パスの解決基点。
- 出力は stdout。差分検査（`-check`）は利用側の CI で `diff` すればよいので持たない。

`csdfhelp` に登録し、`README.md` に「Promotion」節を追加する。

## 6. パッケージ構成

```
csdf/                 既存。Parser に 3 宣言を追加、Diagram に Promotes/Syncs/Constrains、HasDirectives()
csdf/promote/         展開・lint。Expand(global Diagram, loader func(path) (Diagram, error), opts) (Diagram, []Diagnostic, error)
tools/csdfpromote/    CLI
docs/PROMOTION.md     意味論・CSP/Z 対応・tau の扱い・置換しない判断・sync の引数結合規則・in 節と凍結
examples/promote/     golden test の入力と期待出力
```

他ツールへの `HasDirectives()` チェックは、各 CLI が共通で使っている読み込みヘルパに 1 箇所入れる（ヘルパがなければ作り、各 CLI から呼ぶ）。

## 7. テスト

1. **パーサ**：3 宣言の受理・拒否（引数欠落、閉じ括弧欠落、宣言の位置、`in` 節あり・なし・複数）。既存の golden test がすべて通ること（後方互換）。
2. **展開 golden test**（`examples/promote/`）：
   - 単一局所・状態変数なし（`A → B → STOP`）
   - 状態変数あり、生成・削除・自己ループ・非決定分岐
   - `tau` を含む局所
   - 2 局所 + `sync`（直積が 1×1、2×1、2×2）
   - `constrain` が生成・通常・sync 済みエッジに当たる
   - 手書き大局エッジとの混在
   - `in` を 2 状態指定（同じ展開が両状態に複製され、手書きのモード切替エッジが残る）
   - 日本語テンプレート
3. **lint テスト**：4.6 の各行に 1 ケース。
4. **他ツールの拒否**：宣言入りの図を各ツールに渡してエラーメッセージを確認。
5. **列挙形との相互検算**（Phase 2）：有限 ID（2〜3 個）で `csdfrename` + `csdfparallel` の直積を作り、`csdfrefinement -m f` の義務を両方向に生成する。述語が不透明なので自動判定はしないが、状態変数のない局所については義務が自明に閉じることを確認する。
6. **`csdfrepl`**：展開結果を 0-switch でアニメーションできる（写像の値は JSON オブジェクト）。

## 8. マイルストーン

| # | 内容 | 受け入れ条件 |
|---|---|---|
| M1 | 文法・AST・`csdfparse` JSON・他ツールの拒否 | 既存 golden test 全通過。宣言入り図を `csdfparse` が JSON で出し、他ツールが拒否する |
| M2 | `promote` のみの展開（sync/constrain なし）、`in` 節、lint の error 群 | 単一局所の golden test 通過。`in` を 2 状態指定した golden test 通過。`csdflivelockfree` / `csdfrepl` に通る |
| M3 | `sync` / `constrain` / warning・info 群 / `-template` | 全 golden test 通過 |
| M4 | ドキュメント、`csdfhelp`、README、SYNTAX.md、GLOSSARY.md | `csdfhelp csdfpromote` が出る。SYNTAX.md の ABNF が実装と一致 |
| M5（Phase 2） | `csdfrename`、列挙形、相互検算 | 有限 ID で両形が出て、義務ファイルが生成される |

## 9. 決定事項

1. **イベント引数の記法**：`e(id, a, b)` を採用する（既存の利用側スクリプトと一致）。`e.id` は採らない。イベントは自由文字列なので、これは規約であって文法ではない。
2. **局所変数は置換しない**。置換案（`v` → `m(id).v`）は述語が不透明である以上、字面置換で壊れうる。展開後の述語に裸の `v` が残るが、直前の由来コメントで文脈が判る。
3. **`sync` の引数結合**：名前一致で重複を潰す。名前が同じで意味が違う引数があると誤結合するので、docs に注意を書く。
4. **`tau` に ID を付けない**。付ければ観測可能になり、隠蔽の意味が変わる。
5. **削除時に `p` を捨てる**。`S₀` が変数を持たないので捨ててよい。`p` が空でなければ warning を出す。
6. **複数状態への `promote`**：`in <stateID>, …` を最初から文法に入れる。省略時は `startEdgeDecl.dst`。挙げた各状態に同じ展開を複製する。`in` に挙げていないが `<map>` を持つ状態は「凍結」として info で読み上げる。`sync` は対象写像の `in` 集合の交わりに展開し、交わりが空なら lint error。`constrain` は状態に依存しない。モード切替のエッジとそのフレームは手書きで、生成しない。

## 10. 参考：期待する使い方

```plantuml
@startuml CUSTODY-SPEC-STATE
state "稼働中" as running
running : whitelistEntries ; エントリID ⇸ Whitelist
running : buyTrades        ; 買い約定ID ⇸ BuyTrade
running : segReportCycles  ; 基準日 ⇸ SegReport
running : sessions         ; ユーザー ⇸ Session

[*] --> running : すべての写像は空

promote states/WHITELIST_ja.puml   as Whitelist via whitelistEntries(エントリID)
promote states/BUY_ja.puml         as BuyTrade  via buyTrades(買い約定ID)
promote states/SEGREPORT_ja.puml   as SegReport via segReportCycles(基準日)
promote states/SESSION_ja.puml     as Session   via sessions(ユーザー)

sync EVT-CUSTODY-DEPOSIT-UNKNOWN-BOOK : buyTrades(買い約定ID), segReportCycles(基準日)

constrain EVT-HW-WHITELIST-VERIFY(売り約定ID, 送金先アドレス) ; whitelistEntries の 送金先アドレス が〈登録済み〉
constrain EVT-WL-CHECKER-APPROVE(エントリID, checker) ; sessions の checker が〈ログイン中〉で役割が checker
@enduml
```

```
$ csdfpromote 02_spec/CUSTODY-SPEC-STATE_ja.puml > build/CUSTODY-SPEC-STATE_ja.expanded.puml
$ csdflivelockfree build/CUSTODY-SPEC-STATE_ja.expanded.puml
$ csdfrepl        build/CUSTODY-SPEC-STATE_ja.expanded.puml
```
