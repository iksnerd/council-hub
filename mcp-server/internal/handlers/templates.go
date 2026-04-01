package handlers

import "sort"

// RoomTemplate defines default field values for a named room type.
type RoomTemplate struct {
	Topic        string
	Tags         string
	TechStack    string
	SystemPrompt string
	InitialMsg   string // posted as type=thought from author=system after room creation
}

var roomTemplates = map[string]RoomTemplate{
	"decision-log": {
		Tags:         "decision,architecture",
		SystemPrompt: "Track architectural decisions. Use type=decision for final choices, type=critique for concerns, type=thought for alternatives considered.",
		InitialMsg:   "Decision log initialized.",
	},
	"sprint": {
		Tags:         "sprint,planning",
		SystemPrompt: "Coordinate sprint work. Use type=action for tasks, type=decision for commitments, type=critique for blockers.",
		InitialMsg:   "Sprint room ready.",
	},
	"bug": {
		Tags:         "bug,investigation",
		SystemPrompt: "Track a bug investigation. Use type=thought for hypotheses, type=decision for root cause conclusions, type=action for fix tasks.",
		InitialMsg:   "Bug investigation room open.",
	},
	"brainstorm": {
		Tags:         "brainstorm,exploration",
		SystemPrompt: "Open-ended exploration. Use type=thought for ideas, type=critique for concerns, type=decision when converging.",
		InitialMsg:   "Brainstorm room open.",
	},
	"review": {
		Tags:         "review",
		SystemPrompt: "Coordinate a review. Use type=critique for issues, type=decision for approval/rejection, type=action for required changes.",
		InitialMsg:   "Review room ready.",
	},
}

func templateNames() []string {
	names := make([]string, 0, len(roomTemplates))
	for k := range roomTemplates {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
