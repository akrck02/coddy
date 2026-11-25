package servers

import (
	"testing"
	"time"

	"github.com/akrck02/coddy/editor/internal/lsp"
)

func TestTheStatusSaysWhatAServerDoes(test *testing.T) {
	started := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		progress lsp.Progress
		want     string
	}{
		{lsp.Progress{Phase: "importing", Detail: "harbour.api", Percent: -1, Done: 7, Total: 22, Started: started},
			"kotlin-lsp · importing harbour.api (7 of 22) · 3:12"},
		{lsp.Progress{Phase: "indexing", Detail: "45%", Percent: 45, Started: started}, "kotlin-lsp · indexing 45 % · 3:12"},
		{lsp.Progress{Phase: "starting", Detail: "Importing Gradle project(s)", Percent: -1, Started: started},
			"kotlin-lsp · starting Importing Gradle project(s) · 3:12"},
		{lsp.Progress{Phase: "ready"}, "kotlin-lsp ready"},
	}
	for _, each := range cases {
		if said := StatusOf("kotlin-lsp", each.progress, started.Add(3*time.Minute+12*time.Second)); said != each.want {
			test.Errorf("the status says %q, want %q", said, each.want)
		}
	}
}
