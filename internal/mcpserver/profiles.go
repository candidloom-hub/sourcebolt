package mcpserver

// Tool profiles: "default" exposes every tool except the opt-in "batch"
// group; "minimal" exposes the 8 tools small-context hosts need most;
// "all" exposes everything.
var minimal = map[string]bool{
	"server_info":     true,
	"notebook_list":   true,
	"notebook_open":   true,
	"source_add":      true,
	"source_read":     true,
	"chat_ask":        true,
	"studio_generate": true,
	"studio_list":     true,
}

const groupBatch = "batch"

func enabled(profile, tool, group string) bool {
	switch profile {
	case "all":
		return true
	case "minimal":
		return minimal[tool]
	default:
		return group != groupBatch
	}
}
