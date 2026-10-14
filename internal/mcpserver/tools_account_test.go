package mcpserver

import (
	"encoding/json/v2"
	"testing"
)

const quotaPayload = `[1,[[null,null,null,null,1,[100,0],null,100]]]`

func TestAccountSettingsReadAndSet(t *testing.T) {
	st := newStub()
	st.payloads["EylDcb"] = quotaPayload
	st.payloads["hT54vc"] = `[]`
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "account_settings"); a.ReadOnlyHint || !a.IdempotentHint {
		t.Errorf("annotations = %+v", a)
	}
	var out AccountSettingsOut
	if err := json.Unmarshal([]byte(text(call(t, cs, "account_settings", nil))), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Quota.Windows) != 1 || out.LanguageSet || st.count("hT54vc") != 0 {
		t.Fatalf("read: %+v", out)
	}
	out = AccountSettingsOut{}
	if err := json.Unmarshal([]byte(text(call(t, cs, "account_settings", map[string]any{"output_language": "de"}))), &out); err != nil {
		t.Fatal(err)
	}
	if !out.LanguageSet || out.OutputLanguage != "de" || st.count("hT54vc") != 1 {
		t.Fatalf("set: %+v", out)
	}
}

func TestAccountSettingsError(t *testing.T) {
	st := newStub()
	st.errs["EylDcb"] = authErr("account.quota")
	if e := envelope(t, call(t, connect(t, st, "default"), "account_settings", nil)); e.Code != "AUTH_REQUIRED" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestServerInfoIncludeUsage(t *testing.T) {
	st := newStub()
	st.payloads["EylDcb"] = quotaPayload
	cs := connect(t, st, "default")
	var out ServerInfoOut
	if err := json.Unmarshal([]byte(text(call(t, cs, "server_info", nil))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Usage != nil || st.count("EylDcb") != 0 {
		t.Fatalf("usage fetched without include_usage: %+v", out)
	}
	out = ServerInfoOut{}
	if err := json.Unmarshal([]byte(text(call(t, cs, "server_info", map[string]any{"include_usage": true}))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Usage == nil || len(out.Usage.Windows) != 1 || st.count("EylDcb") != 1 {
		t.Fatalf("out = %+v", out)
	}
}
