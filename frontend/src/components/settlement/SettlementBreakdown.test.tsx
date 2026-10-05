import { expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import SettlementBreakdown from './SettlementBreakdown';
import { expenseFixture } from '../assistant/fixtures';

it('shows hourly shuttle value shares without inventing per-hour shuttle counts', () => {
  const preview = expenseFixture().settlement;
  preview.bands.base = { hours: 2, court_cents: 6000, shuttle_units: 8, shuttle_cents: 2222, total_cents: 8222, heads: 2 };
  preview.bands.extra = { hours: 1, court_cents: 2300, shuttle_units: 0, shuttle_cents: 1111, total_cents: 3411, heads: 1 };
  preview.lines[1].in_extra = false;
  preview.lines[0].amount_cents = 7522;
  preview.lines[1].amount_cents = 4111;
  render(<SettlementBreakdown preview={preview} rates={{ base_hours: 2, base_rate_cents: 3000, extra_hours: 1, extra_rate_cents: 2300, shuttles_per_hour: 2, actual_shuttles: 8 }} showRates />);
  expect(screen.getByText(/2h court.*shuttle share \$22.22/)).toBeInTheDocument();
  expect(screen.getByText(/1h court.*shuttle share \$11.11/)).toBeInTheDocument();
  expect(screen.getByText(/8 shuttles counted for the whole session/)).toBeInTheDocument();
  expect(screen.getByText('left early')).toBeInTheDocument();
  expect(screen.queryByText(/0 shuttles/)).not.toBeInTheDocument();
});
