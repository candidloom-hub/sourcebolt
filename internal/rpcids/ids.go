// Package rpcids maps logical RPC names to NotebookLM's obfuscated wire ids.
// Ids drift when Google ships new web builds, so users can override any of
// them at runtime with a JSON file (see LoadOverrides). Source of the
// defaults: research/rpc-catalog.md (consensus and conflicting rows only;
// stale ids are excluded).
package rpcids

import (
	"maps"
	"sync"
)

var defaults = map[string]string{
	// Notebooks
	"notebooks.list":     "wXbhsf",
	"notebooks.get":      "rLM1Ne",
	"notebooks.create":   "CCqFvf",
	"notebooks.mutate":   "s0tc2d",
	"notebooks.delete":   "WWINqb",
	"notebooks.unrecent": "fejl7e",
	"notebooks.copy":     "te3DCe",
	"notebooks.guide":    "VfAZjd",
	"notebooks.pin":      "LQhfEb",

	// Sources
	"sources.add":             "izAoDd",
	"sources.add_async":       "X1snv",
	"sources.add_url_v2":      "ozz5Z",
	"sources.register_upload": "o4cbdc",
	"sources.delete":          "tGMBJ",
	"sources.load":            "hizoJc",
	"sources.rename":          "b7Wfje",
	"sources.refresh":         "FLmJqe",
	"sources.refresh_batch":   "dtT1F",
	"sources.freshness":       "yR9Yof",
	"sources.append":          "QsNTEd",
	"sources.copy":            "R27wvc",
	"sources.chunks":          "ASU5Oe",
	"sources.guide":           "tr032e",

	// Research
	"research.discover": "Es3dTe",
	"research.fast":     "Ljjv0c",
	"research.deep":     "QA9ei",
	"research.poll":     "e3bVqc",
	"research.import":   "LBwxtb",
	"research.cancel":   "Zbrupe",

	// Chat (the answer itself streams via GenerateFreeFormStreamed, not an id)
	"chat.sessions":     "hPTbtc",
	"chat.turns":        "khqZz",
	"chat.delete_turns": "J7Gthc",
	"chat.status":       "oXwmh",
	"chat.cancel":       "XgrPMd",
	"chat.suggest":      "otmP3b",
	"chat.next_steps":   "OcvKNc",

	// Notes and mind maps
	"notes.create":         "CYK0Xb",
	"notes.list":           "cFji9",
	"notes.mutate":         "cYAfTb",
	"notes.delete":         "AH0mwd",
	"notes.act_on_sources": "yyryJe",

	// Studio artifacts
	"studio.create":             "R7cb6c",
	"studio.list":               "gArtLc",
	"studio.get":                "v9rmvd",
	"studio.delete":             "V5N4be",
	"studio.rename":             "rc3d8d",
	"studio.export":             "Krh3pd",
	"studio.share":              "RGP97b",
	"studio.derive":             "KmcKPe",
	"studio.generate":           "Rytqqe",
	"studio.copy":               "mKDdke",
	"studio.choices":            "sqTeoe",
	"studio.report_suggestions": "ciyUvf",

	// Labels and collections
	"labels.list":   "I3xc3c",
	"labels.create": "agX4Bc",
	"labels.mutate": "le8sX",
	"labels.delete": "GyzE7e",

	// Sharing
	"sharing.share":   "QDyure",
	"sharing.details": "JFMDGd",

	// Account
	"account.get_or_create": "ZwVcOc",
	"account.mutate":        "hT54vc",
	"account.quota":         "EylDcb",
	"account.get":           "SatQRc",
}

var (
	mu  sync.RWMutex
	ids = maps.Clone(defaults)
)

// Lookup returns the wire id for a logical name.
func Lookup(name string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	id, ok := ids[name]
	return id, ok
}

// All returns a copy of the current name-to-id table.
func All() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	return maps.Clone(ids)
}

// Reset restores the built-in defaults.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	ids = maps.Clone(defaults)
}
