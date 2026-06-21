package authorization

type Request struct {
	Action       string
	ResourceType string
	ScopeType    string
	ScopeID      string
}

type Rule struct {
	Action       string
	ResourceType string
	ScopeType    string
	ScopeID      string
	Behavior     string
	Priority     int
	Reason       string
}

type Result struct {
	Allowed     bool
	Behavior    string
	Reason      string
	MatchedRule *Rule
}

func EvaluateRules(request Request, rules []Rule) Result {
	var selected *Rule
	for i := range rules {
		rule := rules[i]
		if !ruleMatches(request, rule) {
			continue
		}
		if selected == nil || compareRule(rule, *selected) > 0 {
			selected = &rules[i]
		}
	}
	if selected == nil {
		return Result{Allowed: defaultAllowed(request.Action), Behavior: defaultBehavior(request.Action), Reason: "no matching rule"}
	}
	behavior := normalizeBehavior(selected.Behavior)
	return Result{
		Allowed:     behavior == "allow" || behavior == "notify",
		Behavior:    behavior,
		Reason:      selected.Reason,
		MatchedRule: selected,
	}
}

func ruleMatches(request Request, rule Rule) bool {
	return matchString(rule.Action, request.Action) &&
		matchString(rule.ResourceType, request.ResourceType) &&
		matchScope(request, rule)
}

func matchScope(request Request, rule Rule) bool {
	if rule.ScopeType == "" || rule.ScopeType == "*" {
		return true
	}
	if rule.ScopeType != request.ScopeType {
		return false
	}
	return rule.ScopeID == "" || rule.ScopeID == "*" || rule.ScopeID == request.ScopeID
}

func matchString(ruleValue, requestValue string) bool {
	return ruleValue == "" || ruleValue == "*" || ruleValue == requestValue
}

func compareRule(left, right Rule) int {
	if leftWeight, rightWeight := scopeWeight(left.ScopeType), scopeWeight(right.ScopeType); leftWeight != rightWeight {
		return leftWeight - rightWeight
	}
	if leftDeny, rightDeny := denyWeight(left.Behavior), denyWeight(right.Behavior); leftDeny != rightDeny {
		return leftDeny - rightDeny
	}
	if left.Priority != right.Priority {
		return left.Priority - right.Priority
	}
	return 0
}

func scopeWeight(scope string) int {
	switch scope {
	case "field":
		return 70
	case "form":
		return 60
	case "function", "feature":
		return 50
	case "project":
		return 40
	case "department":
		return 30
	case "organization":
		return 20
	case "", "*":
		return 0
	default:
		return 10
	}
}

func denyWeight(behavior string) int {
	if normalizeBehavior(behavior) == "deny" {
		return 1
	}
	return 0
}

func normalizeBehavior(behavior string) string {
	switch behavior {
	case "allow", "notify", "approve", "deny":
		return behavior
	default:
		return "deny"
	}
}

func defaultAllowed(action string) bool {
	switch action {
	case "read", "list", "view":
		return true
	default:
		return false
	}
}

func defaultBehavior(action string) string {
	if defaultAllowed(action) {
		return "allow"
	}
	return "deny"
}
