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
		Tags: "decision,architecture",
		SystemPrompt: "This room tracks architectural decisions. " +
			"Post type=thought for alternatives considered, type=critique for concerns and trade-off analysis, " +
			"type=decision for final choices (include rationale), type=synthesis for a compiled ADR when deliberation concludes. " +
			"Link related rooms (e.g. the sprint or bug that prompted this decision) using related_rooms. " +
			"Post a synthesis once the decision is finalized — this becomes the room's lasting reference. " +
			"Pin the synthesis or final decision for newcomer context.",
		InitialMsg: "Decision log initialized. Post thoughts to explore options, then converge with a decision.",
	},
	"sprint": {
		Tags: "sprint,planning",
		SystemPrompt: "This room coordinates sprint work. " +
			"Post type=action for tasks being worked on or shipped, type=decision for commitments and scope choices, " +
			"type=critique for blockers or risks, type=synthesis for a sprint retrospective when the sprint closes. " +
			"Link related rooms (bug rooms, decision-log rooms) using related_rooms. " +
			"Archive with delete=true when the sprint is complete and retro is posted.",
		InitialMsg: "Sprint room ready. Post actions as work progresses, decisions for scope calls.",
	},
	"bug": {
		Tags: "bug,investigation",
		SystemPrompt: "This room tracks a single bug investigation. " +
			"Post type=thought for hypotheses, type=decision when root cause is confirmed (include evidence), " +
			"type=action for fixes shipped, type=synthesis to summarize cause/fix/follow-up when resolved. " +
			"Always link the parent tracker room and related rooms via related_rooms. " +
			"Archive when the fix is verified in production.",
		InitialMsg: "Bug investigation room open. Post thoughts for hypotheses, decision when root cause is confirmed.",
	},
	"brainstorm": {
		Tags: "brainstorm,exploration",
		SystemPrompt: "This room is for open-ended exploration. " +
			"Post type=thought for ideas and possibilities, type=critique for concerns or feasibility issues, " +
			"type=decision when the group converges on a direction, type=synthesis to compile the outcome and next steps. " +
			"Link related rooms for context using related_rooms. " +
			"When exploration yields a concrete plan, create a new decision-log or sprint room and link it here.",
		InitialMsg: "Brainstorm room open. Post thoughts freely, critique to pressure-test ideas.",
	},
	"review": {
		Tags: "review",
		SystemPrompt: "This room coordinates a review (code, design, proposal, etc.). " +
			"Post type=critique for issues found, type=review for general feedback, " +
			"type=decision for approval or rejection (include conditions if any), type=action for required changes. " +
			"Post type=synthesis when the review concludes to summarize outcome and action items. " +
			"Link the room being reviewed via related_rooms.",
		InitialMsg: "Review room ready. Post critiques for issues, decision for final verdict.",
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
