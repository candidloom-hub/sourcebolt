package main

import (
	"encoding/json/v2"
	"net/url"
	"strings"
	"testing"
)

const nbID = "a1b2c3d4-e5f6-4789-abcd-0123456789ab"

func harWith(t *testing.T) []byte {
	t.Helper()
	params := `[null,1,null,[2]]`
	freq := `[[["wXbhsf",` + mustJSON(t, params) + `,null,"generic"]]]`
	inner := `[[["Secret Board Plan",[],"` + nbID + `","",null,[1]],["Other",[],"b2c3d4e5-f6a7-4890-bcde-123456789abc","",null,[2]]],"owner@corp.example"]`
	row := `[["wrb.fr","wXbhsf",` + mustJSON(t, inner) + `,null,null,null,"generic"],["af.httprm",1,"-4242",3]]`
	body := ")]}'\n\n" + "999\n" + row + "\n"
	har := map[string]any{"log": map[string]any{"entries": []any{
		map[string]any{
			"startedDateTime": "2026-10-15T09:30:00.000Z",
			"request": map[string]any{
				"method": "POST",
				"url":    "https://notebook.google.com/_/LabsTailwindUi/data/batchexecute?rpcids=wXbhsf&source-path=%2F&f.sid=-4242&bl=boq_x&rt=c",
				"headers": []any{
					map[string]any{"name": "cookie", "value": "SID=sid-secret-value; __Secure-1PSIDTS=ts-secret"},
					map[string]any{"name": "authorization", "value": "SAPISIDHASH 1760_deadbeefcafe"},
				},
				"postData": map[string]any{"text": "f.req=" + url.QueryEscape(freq) + "&at=AJpM-csrf-secret%3A1760&"},
			},
			"response": map[string]any{"status": 200, "content": map[string]any{"text": body}},
		},
		map[string]any{
			"startedDateTime": "2026-10-15T09:30:01.000Z",
			"request":         map[string]any{"method": "GET", "url": "https://notebook.google.com/static/x.js"},
			"response":        map[string]any{"status": 200, "content": map[string]any{"text": "ignored"}},
		},
	}}}
	b, err := json.Marshal(har)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustJSON(t *testing.T, s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSanitizeStripsSecrets(t *testing.T) {
	fixtures, err := Convert(harWith(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("want 1 fixture (static asset ignored), got %d", len(fixtures))
	}
	out, _ := json.Marshal(fixtures[0])
	s := string(out)
	for _, secret := range []string{"sid-secret-value", "ts-secret", "deadbeefcafe", "AJpM-csrf-secret", "owner@corp.example", "Secret Board Plan", nbID} {
		if strings.Contains(s, secret) {
			t.Errorf("fixture leaks %q", secret)
		}
	}
	f := fixtures[0]
	if f.RPC != "wXbhsf" || f.Synthetic || f.Host != "notebook.google.com" || f.Captured != "2026-10-15" {
		t.Fatalf("metadata = %+v", f)
	}
	if !strings.Contains(s, "user@example.com") {
		t.Error("email must be replaced by user@example.com")
	}
}

func TestSanitizeStableIDs(t *testing.T) {
	fixtures, err := Convert(harWith(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	body := fixtures[0].ResponseBody
	fake := FakeUUID(1)
	if strings.Count(body, fake) != 1 {
		t.Fatalf("first id must map to %s once: %s", fake, body)
	}
	again, _ := Convert(harWith(t), Options{})
	if again[0].ResponseBody != body {
		t.Fatal("sanitizing must be deterministic")
	}
}

func TestKeepTitles(t *testing.T) {
	fixtures, err := Convert(harWith(t), Options{KeepTitles: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fixtures[0].ResponseBody, "Secret Board Plan") {
		t.Fatal("--keep-titles must keep free text")
	}
	if strings.Contains(fixtures[0].ResponseBody, "owner@corp.example") {
		t.Fatal("emails are always replaced")
	}
}
