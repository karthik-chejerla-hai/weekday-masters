import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import TopupForm from './TopupForm';
import { api } from '../../services/api';
import type { PlayerBalance } from '../../types';

vi.mock('../../services/api', () => ({
  api: { recordTopup: vi.fn() },
}));

const members: PlayerBalance[] = [
  { user_id: 'u1', name: 'Karthik', balance_cents: 4250 },
  { user_id: 'u2', name: 'Priya', balance_cents: 3100 },
];

function renderForm(onRecorded = vi.fn()) {
  render(<TopupForm members={members} onRecorded={onRecorded} />);
  return { onRecorded };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.recordTopup).mockResolvedValue({} as never);
});

describe('TopupForm', () => {
  // The whole point of this form: a typed dollar amount becomes integer cents
  // exactly once, here, before anything else touches it.
  it('converts the typed amount to integer cents', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.selectOptions(screen.getByLabelText('Member'), 'u1');
    await user.type(screen.getByLabelText('Amount'), '50.00');
    await user.click(screen.getByRole('button', { name: 'Record top-up' }));

    await waitFor(() => expect(api.recordTopup).toHaveBeenCalledWith('u1', 5000, 'Bank transfer'));
  });

  it('handles an amount that a float would round badly', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.selectOptions(screen.getByLabelText('Member'), 'u2');
    await user.type(screen.getByLabelText('Amount'), '16.94');
    await user.click(screen.getByRole('button', { name: 'Record top-up' }));

    await waitFor(() => expect(api.recordTopup).toHaveBeenCalledWith('u2', 1694, 'Bank transfer'));
  });

  it('sends the note when one is given', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.selectOptions(screen.getByLabelText('Member'), 'u1');
    await user.type(screen.getByLabelText('Amount'), '25');
    await user.type(screen.getByLabelText('Note'), 'PayID from Priya');
    await user.click(screen.getByRole('button', { name: 'Record top-up' }));

    await waitFor(() =>
      expect(api.recordTopup).toHaveBeenCalledWith('u1', 2500, 'PayID from Priya')
    );
  });

  it('will not submit without a member or an amount', async () => {
    const user = userEvent.setup();
    renderForm();

    const submit = screen.getByRole('button', { name: 'Record top-up' });
    expect(submit).toBeDisabled();

    await user.selectOptions(screen.getByLabelText('Member'), 'u1');
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText('Amount'), '50');
    expect(submit).toBeEnabled();
  });

  it('refuses a zero or negative amount', async () => {
    const user = userEvent.setup();
    renderForm();

    await user.selectOptions(screen.getByLabelText('Member'), 'u1');
    await user.type(screen.getByLabelText('Amount'), '0');

    expect(screen.getByRole('button', { name: 'Record top-up' })).toBeDisabled();
    expect(api.recordTopup).not.toHaveBeenCalled();
  });

  it('clears itself and tells the page once the top-up is recorded', async () => {
    const user = userEvent.setup();
    const { onRecorded } = renderForm();

    await user.selectOptions(screen.getByLabelText('Member'), 'u1');
    await user.type(screen.getByLabelText('Amount'), '50');
    await user.click(screen.getByRole('button', { name: 'Record top-up' }));

    await waitFor(() => expect(onRecorded).toHaveBeenCalled());
    expect(screen.getByLabelText('Amount')).toHaveValue(null);
    expect(screen.getByLabelText('Member')).toHaveValue('');
  });

  // A failed top-up must say so rather than looking like it worked — the admin
  // would otherwise believe money was recorded that was not.
  it('reports a failure instead of silently doing nothing', async () => {
    vi.mocked(api.recordTopup).mockRejectedValue(new Error('network'));
    const user = userEvent.setup();
    const { onRecorded } = renderForm();

    await user.selectOptions(screen.getByLabelText('Member'), 'u1');
    await user.type(screen.getByLabelText('Amount'), '50');
    await user.click(screen.getByRole('button', { name: 'Record top-up' }));

    await waitFor(() =>
      expect(screen.getByText(/Could not record that top-up/)).toBeInTheDocument()
    );
    expect(onRecorded).not.toHaveBeenCalled();
  });
});
