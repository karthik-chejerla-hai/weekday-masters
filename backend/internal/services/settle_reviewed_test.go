package services

import "github.com/weekday-masters/backend/internal/models"

// settleReviewed settles the way the settlement form does: cost a preview, then
// confirm it with that preview's fingerprint.
//
// Settle refuses to post without a reviewed fingerprint, so tests that are about
// something else (splits, locks, notifications) go through this rather than
// each repeating the two steps. A test that sets ExpectedPreview itself is
// exercising the review rule, and its value is passed through untouched.
func settleReviewed(s *SettlementService, in SettleInput) (*models.Settlement, *SettlementPreview, error) {
	if in.ExpectedPreview == "" {
		preview, err := s.Preview(in)
		if err != nil {
			return nil, nil, err
		}
		in.ExpectedPreview = preview.Fingerprint
	}
	return s.Settle(in)
}
