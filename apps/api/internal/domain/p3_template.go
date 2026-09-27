package domain

// P3Template is adapted from the P3.express v2 manual (OMIMO, CC BY 4.0).
// https://omimo.org/en/modules/p3.express/manual/v2/
var P3Template = []struct {
	Code, Name string
	Steps      []string
}{
	{"A", "Project Initiation", []string{"Appoint the sponsor", "Appoint the project manager", "Appoint the key team members", "Describe the project", "Identify and plan the deliverables", "Identify risks and plan responses", "Have project initiation peer-reviewed", "Make a go/no-go decision", "Kick off the project", "Conduct a focused communication"}},
	{"B", "Monthly Initiation", []string{"Revise and refine the plans", "Have the monthly cycle peer-reviewed", "Make a go/no-go decision", "Kick off the monthly cycle", "Conduct a focused communication"}},
	{"C", "Weekly Management", []string{"Measure and report performance", "Plan responses for deviations", "Kick off the weekly cycle", "Conduct a focused communication"}},
	{"D", "Daily Management", []string{"Manage risks, issues, and change requests", "Accept completed deliverables"}},
	{"E", "Monthly Closure", []string{"Evaluate stakeholder satisfaction", "Capture lessons and plan for improvements", "Conduct a focused communication"}},
	{"F", "Project Closure", []string{"Hand over the product", "Evaluate stakeholder satisfaction", "Have the closing activity group peer-reviewed", "Archive the project documents", "Celebrate!", "Conduct a focused communication"}},
	{"G", "Post-Project Management", []string{"Evaluate the benefits", "Generate new ideas", "Conduct a focused communication"}},
}

var SDLCNames = []string{"Discovery / Analysis", "Design", "Development", "Testing", "Release / Rollout", "Support / Improvement"}
