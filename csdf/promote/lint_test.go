package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

func TestExpandRefusesAnUnsoundPromotion(t *testing.T) {
	const wellFormedLocal = `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
[*] --> accNone
accNone --> accOpen : OPEN
accOpen --> accNone : CLOSE
@enduml
`

	cases := map[string]struct {
		global string
		locals map[string]string
		want   string
	}{
		"a map that the state does not hold": {
			global: `@startuml A
state "稼働中" as running {
  running : audits ; 監査ID ⇸ Audit

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": wellFormedLocal},
			want:   `error: line 5: state "running" holds no state variable named "accounts"`,
		},
		"a promotion outside any state": {
			global: `@startuml A
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account
state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
  !include local/ACCOUNT.puml
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": wellFormedLocal},
			want:   `error: line 4: the <<promote>> block of "accounts" is not inside a state; write the state it moves in as a composite state around it`,
		},
		"the same map promoted twice in one state": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
  state "accounts : 口座ID ⇸ Account" as a2 <<promote>>
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": wellFormedLocal},
			want:   `error: line 8: "accounts" is promoted twice in state "running"`,
		},
		"a map with no local diagram": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>>
}
[*] --> running
@enduml
`,
			want: `error: line 5: no <<promote>> block of "accounts" carries an !include, so it has no local diagram`,
		},
		"two blocks of one map that both include": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
state "縮退中" as degraded {
  degraded : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a2 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": wellFormedLocal},
			want:   `error: line 12: "accounts" already includes its local diagram at line 5; a second !include of one file collides with the first on every state ID`,
		},
		"blocks of one map that disagree": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
state "縮退中" as degraded {
  degraded : accounts ; 口座ID ⇸ Account

  state "accounts : 顧客ID ⇸ Account" as a2 <<promote>>
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": wellFormedLocal},
			want:   `error: line 12: "accounts" is written as "顧客ID ⇸ Account" here and as "口座ID ⇸ Account" at line 5`,
		},
		"a local diagram whose start state holds a variable": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
accNone : balance ; 残高
state "開設済み" as accOpen
[*] --> accNone
accNone --> accOpen : OPEN
@enduml
`},
			want: `error: line 5: the start state "accNone" of local/ACCOUNT.puml holds state variables; it means that the instance does not exist`,
		},
		"a local diagram with an end edge": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
[*] --> accNone
accNone --> accOpen : OPEN
accOpen --> [*]
@enduml
`},
			want: `error: line 5: local/ACCOUNT.puml has an end edge; write a state with no transition out of it instead`,
		},
		"a local diagram whose start state loops onto itself": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
@enduml
`,
			locals: map[string]string{"local/ACCOUNT.puml": `@startuml ACCOUNT
state "未開設" as accNone
state "開設済み" as accOpen
[*] --> accNone
accNone --> accNone : POKE
accNone --> accOpen : OPEN
@enduml
`},
			want: `error: line 5: local/ACCOUNT.puml has an edge from its start state "accNone" back to itself, which is an event of an instance that does not exist`,
		},
		"two local diagrams that share a state ID": {
			global: `@startuml A
state "稼働中" as running {
  running : accounts ; 口座ID ⇸ Account
  running : audits ; 監査ID ⇸ Audit

  state "accounts : 口座ID ⇸ Account" as a1 <<promote>> {
    !include local/ACCOUNT.puml
  }
  state "audits : 監査ID ⇸ Audit" as a2 <<promote>> {
    !include local/AUDIT.puml
  }
}
[*] --> running
@enduml
`,
			locals: map[string]string{
				"local/ACCOUNT.puml": wellFormedLocal,
				"local/AUDIT.puml": `@startuml AUDIT
state "未着手" as accNone
state "調査中" as audOpen
[*] --> accNone
accNone --> audOpen : AUDIT-BEGIN
@enduml
`,
			},
			want: `error: line 9: local/AUDIT.puml and local/ACCOUNT.puml both declare the state "accNone"; !include drops every local diagram into one namespace`,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, diags := expand(t, c.global, c.locals)
			if x != nil {
				t.Errorf("promote.Expand() expansion = %q, want none", x.String())
			}
			if !hasDiagnostic(diags, c.want) {
				t.Errorf("promote.Expand() diagnostics = %v, want one of them to be %q", diags, c.want)
			}
		})
	}
}

func hasDiagnostic(diags []promote.Diagnostic, want string) bool {
	for _, d := range diags {
		if strings.Contains(d.String(), want) {
			return true
		}
	}
	return false
}
