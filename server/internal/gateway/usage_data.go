package gateway

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Payloads are original tool data, never HTTP headers or upstream configuration.
func callData(args json.RawMessage, result *mcp.CallToolResult) *store.CallData {
	output, err := json.Marshal(result)
	if err != nil {
		output = nil
	}
	input, it := limitedPayload(args)
	out, ot := limitedPayload(output)
	return &store.CallData{InputData: input, OutputData: out, InputTruncated: it, OutputTruncated: ot}
}

func limitedPayload(raw []byte) (*string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	const limit = 64 << 10
	truncated := len(raw) > limit
	if truncated {
		end := limit
		for end > 0 && !utf8.RuneStart(raw[end]) {
			end--
		}
		raw = raw[:end]
	}
	value := string(raw)
	return &value, truncated
}
