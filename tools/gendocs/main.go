// Command gendocs prints the MCP tool reference (docs/tools.md) as Markdown.
//
//	go run ./tools/gendocs > docs/tools.md
//
// It builds the real server in-process with stub dependencies (no network,
// no credentials), lists tools through an in-memory MCP client under each
// tool profile, and renders name, profile membership, description, input
// parameters and annotations. A test fails when docs/tools.md is stale.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// noNet is a transport that refuses every call, so generating docs can
// never reach Google.
type noNet struct{}

func (noNet) Name() string         { return "none" }
func (noNet) Supports(string) bool { return false }
func (noNet) Do(context.Context, transport.Call) (transport.Result, error) {
	return transport.Result{}, fmt.Errorf("gendocs: network disabled")
}

func listTools(ctx context.Context, profile string) ([]*mcp.Tool, error) {
	srv := mcpserver.New(mcpserver.Deps{
		NLM:     nlm.New(noNet{}),
		Cache:   cache.New(nil),
		Profile: "default",
		Host:    "notebook.google.com",
	}, profile)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "gendocs", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()
	var out []*mcp.Tool
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func names(ts []*mcp.Tool) map[string]bool {
	m := map[string]bool{}
	for _, t := range ts {
		m[t.Name] = true
	}
	return m
}

// groupOf is the tool-name prefix ("source_add" -> "source").
func groupOf(name string) string {
	if i := strings.IndexByte(name, '_'); i > 0 {
		return name[:i]
	}
	return name
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "|", `\|`)), " ")
}

func typeOf(p map[string]any) string {
	t, _ := p["type"].(string)
	if ts, ok := p["type"].([]any); ok {
		var parts []string
		for _, x := range ts {
			parts = append(parts, fmt.Sprint(x))
		}
		t = strings.Join(parts, " or ")
	}
	if t == "array" {
		if it, ok := p["items"].(map[string]any); ok {
			if it2 := typeOf(it); it2 != "any" {
				t = "array of " + it2
			}
		}
	}
	if t == "" {
		t = "any"
	}
	if en, ok := p["enum"].([]any); ok {
		var vs []string
		for _, e := range en {
			vs = append(vs, fmt.Sprintf("`%v`", e))
		}
		t += " (" + strings.Join(vs, ", ") + ")"
	}
	return t
}

func profilesOf(inDefault, inMinimal bool) string {
	var p []string
	if inMinimal {
		p = append(p, "minimal")
	}
	if inDefault {
		p = append(p, "default")
	} else {
		p = append(p, "batch")
	}
	p = append(p, "all")
	return strings.Join(p, ", ")
}

func renderTool(b *bytes.Buffer, t *mcp.Tool, inDefault, inMinimal bool) {
	fmt.Fprintf(b, "### `%s`\n\n", t.Name)
	fmt.Fprintf(b, "Profiles: %s.\n\n", profilesOf(inDefault, inMinimal))
	if t.Description != "" {
		fmt.Fprintf(b, "%s\n\n", strings.TrimSpace(t.Description))
	}
	if a := t.Annotations; a != nil {
		var parts []string
		if a.Title != "" {
			parts = append(parts, "title: "+a.Title)
		}
		parts = append(parts, fmt.Sprintf("readOnly: %t", a.ReadOnlyHint))
		if a.DestructiveHint != nil {
			parts = append(parts, fmt.Sprintf("destructive: %t", *a.DestructiveHint))
		} else if !a.ReadOnlyHint {
			parts = append(parts, "destructive: true (MCP default)")
		}
		parts = append(parts, fmt.Sprintf("idempotent: %t", a.IdempotentHint))
		if a.OpenWorldHint != nil {
			parts = append(parts, fmt.Sprintf("openWorld: %t", *a.OpenWorldHint))
		} else {
			parts = append(parts, "openWorld: true (MCP default)")
		}
		fmt.Fprintf(b, "Annotations: %s.\n\n", strings.Join(parts, ", "))
	}
	schema, _ := t.InputSchema.(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		b.WriteString("Parameters: none.\n\n")
		return
	}
	req := map[string]bool{}
	if rs, ok := schema["required"].([]any); ok {
		for _, r := range rs {
			req[fmt.Sprint(r)] = true
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if req[keys[i]] != req[keys[j]] {
			return req[keys[i]]
		}
		return keys[i] < keys[j]
	})
	b.WriteString("| Parameter | Type | Required | Description |\n|---|---|---|---|\n")
	for _, k := range keys {
		p, _ := props[k].(map[string]any)
		desc, _ := p["description"].(string)
		r := "no"
		if req[k] {
			r = "yes"
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n", k, oneLine(typeOf(p)), r, oneLine(desc))
		// One level of nested object properties, as `parent.child` rows.
		sub, _ := p["properties"].(map[string]any)
		subKeys := make([]string, 0, len(sub))
		for sk := range sub {
			subKeys = append(subKeys, sk)
		}
		sort.Strings(subKeys)
		for _, sk := range subKeys {
			sp, _ := sub[sk].(map[string]any)
			sdesc, _ := sp["description"].(string)
			fmt.Fprintf(b, "| `%s.%s` | %s | no | %s |\n", k, sk, oneLine(typeOf(sp)), oneLine(sdesc))
		}
	}
	b.WriteString("\n")
}

// render builds the whole document.
func render(ctx context.Context) (string, error) {
	all, err := listTools(ctx, "all")
	if err != nil {
		return "", err
	}
	def, err := listTools(ctx, "default")
	if err != nil {
		return "", err
	}
	min, err := listTools(ctx, "minimal")
	if err != nil {
		return "", err
	}
	inDef, inMin := names(def), names(min)

	groups := map[string][]*mcp.Tool{}
	for _, t := range all {
		g := groupOf(t.Name)
		groups[g] = append(groups[g], t)
	}
	order := make([]string, 0, len(groups))
	for g := range groups {
		order = append(order, g)
		sort.Slice(groups[g], func(i, j int) bool { return groups[g][i].Name < groups[g][j].Name })
	}
	sort.Strings(order)

	var b bytes.Buffer
	b.WriteString("<!-- Generated by `go run ./tools/gendocs > docs/tools.md`. Do not edit by hand. -->\n\n")
	b.WriteString("# Tool reference\n\n")
	fmt.Fprintf(&b, "sourcebolt exposes %d tools with `--tools all`, %d by default and %d with `--tools minimal`. ", len(all), len(def), len(min))
	b.WriteString("The `batch` tools are opt-in: they are listed only under `--tools all`. Tools are grouped by name prefix.\n\n")
	b.WriteString("Every tool returns structured JSON. A failure comes back as a JSON error envelope with a stable error kind.\n\n")
	b.WriteString("## Index\n\n| Tool | Profiles | Summary |\n|---|---|---|\n")
	for _, g := range order {
		for _, t := range groups[g] {
			s := oneLine(t.Description)
			if i := strings.Index(s, ". "); i > 0 {
				s = s[:i+1]
			}
			fmt.Fprintf(&b, "| [`%s`](#%s) | %s | %s |\n", t.Name, t.Name, profilesOf(inDef[t.Name], inMin[t.Name]), s)
		}
	}
	b.WriteString("\n")
	for _, g := range order {
		fmt.Fprintf(&b, "## %s\n\n", g)
		for _, t := range groups[g] {
			renderTool(&b, t, inDef[t.Name], inMin[t.Name])
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func main() {
	out, err := render(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
	fmt.Print(out)
}
