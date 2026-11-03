package capability

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

type Router struct {
	repo *Repository
}

func NewRouter(repo *Repository) *Router {
	return &Router{repo: repo}
}

type MatchRequest struct {
	TaskDescription string         `json:"task_description"`
	MVRUID          uuid.UUID      `json:"mvru_id,omitempty"`
	RequiredLevel   string         `json:"required_level,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type RankedCapability struct {
	Capability Capability `json:"capability"`
	Score      float64    `json:"score"`
	Reason     string     `json:"reason"`
}

func (r *Router) MatchTask(ctx context.Context, req MatchRequest) ([]RankedCapability, error) {
	caps, err := r.repo.ListCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	var ranked []RankedCapability
	desc := strings.ToLower(req.TaskDescription)

	for _, cap := range caps {
		if !cap.IsActive {
			continue
		}
		if req.RequiredLevel != "" && permissionLevelWeight(cap.PermissionLevel) < permissionLevelWeight(req.RequiredLevel) {
			continue
		}

		score := scoreCapability(cap, desc)
		if score > 0 {
			ranked = append(ranked, RankedCapability{
				Capability: cap,
				Score:      score,
				Reason:     scoreReason(score),
			})
		}
	}

	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].Score > ranked[i].Score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	if len(ranked) > 10 {
		ranked = ranked[:10]
	}
	return ranked, nil
}

func scoreCapability(cap Capability, desc string) float64 {
	nameScore := 0.0
	if strings.Contains(desc, strings.ToLower(cap.Name)) {
		nameScore = 0.5
	}

	descScore := 0.0
	capDesc := strings.ToLower(cap.Description)
	words := strings.Fields(desc)
	matchCount := 0
	for _, w := range words {
		if len(w) > 3 && strings.Contains(capDesc, w) {
			matchCount++
		}
	}
	if len(words) > 0 {
		descScore = float64(matchCount) / float64(len(words))
	}

	return math.Min(nameScore+descScore, 1.0)
}

func scoreReason(score float64) string {
	switch {
	case score >= 0.8:
		return "high match"
	case score >= 0.5:
		return "good match"
	case score >= 0.2:
		return "partial match"
	default:
		return "low match"
	}
}

func permissionLevelWeight(level string) int {
	switch level {
	case "L1":
		return 1
	case "L2":
		return 2
	case "L3":
		return 3
	case "L4":
		return 4
	default:
		return 0
	}
}
