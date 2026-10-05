import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import ExpenseReview from './ExpenseReview';
import { expenseFixture } from './fixtures';
import { api } from '../../services/api';
vi.mock('../../services/api', () => ({ api: { confirmExpense: vi.fn(), previewExpense: vi.fn() } }));
beforeEach(() => vi.clearAllMocks());
it('refreshes changed charges without confirming them automatically', async () => {
  const next = { ...expenseFixture(), fingerprint: 'b'.repeat(64) };
  vi.mocked(api.confirmExpense).mockRejectedValue({ response: { data: { code: 'preview_changed', message: 'The expense has changed. Review a new preview.' } } });
  vi.mocked(api.previewExpense).mockResolvedValue(next);
  const refresh = vi.fn(); const user = userEvent.setup();
  render(<MemoryRouter><ExpenseReview preview={expenseFixture()} onRefresh={refresh} /></MemoryRouter>);
  await user.click(screen.getByRole('button', { name: /Confirm expense/ }));
  expect(await screen.findByRole('alert')).toHaveTextContent('Review a new preview');
  expect(refresh).toHaveBeenCalledWith(next);
  expect(api.confirmExpense).toHaveBeenCalledTimes(1);
});
it('blocks repeated clicks while confirmation is running', async () => {
  vi.mocked(api.confirmExpense).mockReturnValue(new Promise(() => {}));
  const user = userEvent.setup(); render(<MemoryRouter><ExpenseReview preview={expenseFixture()} onRefresh={vi.fn()} /></MemoryRouter>);
  await user.dblClick(screen.getByRole('button', { name: /Confirm expense/ }));
  expect(api.confirmExpense).toHaveBeenCalledTimes(1);
});
