package runlog

import (
	"encoding/json"
	"strings"
)

func ReadEvents(text string) ([]map[string]any, error) {
	var events []map[string]any
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
