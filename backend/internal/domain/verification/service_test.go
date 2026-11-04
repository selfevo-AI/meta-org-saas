package verification

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestCalculateOverallScoreWeightsAndNilHandling(t *testing.T) {
	value := func(v float64) *float64 { return &v }

	if score := calculateOverallScore(nil, nil, nil); score != nil {
		t.Fatalf("all-nil score = %v, want nil so no overall score is stored", *score)
	}

	cases := []struct {
		name                      string
		result, path, environment *float64
		want                      float64
	}{
		{"all perfect", value(1), value(1), value(1), 1.0},
		{"result only", value(1), nil, nil, 0.4},
		{"path only", nil, value(1), nil, 0.35},
		{"environment only", nil, nil, value(1), 0.25},
		{"half result", value(0.5), nil, nil, 0.2},
		{"mixed", value(1), value(0.5), value(0), 0.575},
	}
	for _, tc := range cases {
		score := calculateOverallScore(tc.result, tc.path, tc.environment)
		if score == nil {
			t.Fatalf("%s: score = nil, want %v", tc.name, tc.want)
		}
		if *score != tc.want {
			t.Errorf("%s: score = %v, want %v", tc.name, *score, tc.want)
		}
	}
}

func TestAssignReviewValidatesLevelAndReviewerType(t *testing.T) {
	service := NewService(nil)

	if _, err := service.AssignReview(context.Background(), AssignReviewInput{ReportID: uuid.New(), Level: "L4", ReviewerType: "machine"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid level error = %v, want ErrValidation", err)
	}
	if _, err := service.AssignReview(context.Background(), AssignReviewInput{ReportID: uuid.New(), Level: "", ReviewerType: "machine"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("empty level error = %v, want ErrValidation", err)
	}
	if _, err := service.AssignReview(context.Background(), AssignReviewInput{ReportID: uuid.New(), Level: "L2", ReviewerType: "human"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid reviewer_type error = %v, want ErrValidation", err)
	}
	// The valid payload path delegates to the repository, which needs a live
	// database; it is exercised by the route-level integration flows instead.
}
