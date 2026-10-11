import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AxiosError } from 'axios';
import { api } from '../../services/api';
import BalanceCorrectionForm from './BalanceCorrectionForm';
import type { ClubPosition } from '../../types';

vi.mock('../../services/api', () => ({ api: { correctAssetBalance: vi.fn(), listBalanceCorrections: vi.fn() } }));
const position = { assets: { bank_cents: 60661, court_credit_cents: 5300 }, surplus_cents: 21721 } as ClubPosition;

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listBalanceCorrections).mockResolvedValue({items:[],total:0});
  vi.mocked(api.correctAssetBalance).mockResolvedValue({} as never);
});

async function review(amount = '113.00') {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText('Correct balance (AUD)'), amount);
  await user.type(screen.getByLabelText('Reason'), 'Verified venue statement');
  await user.click(screen.getByRole('button', {name:'Review correction'}));
  return user;
}

describe('BalanceCorrectionForm', () => {
  it('reviews before posting and preserves the reason in an audit record', async () => {
    const recorded = vi.fn();
    render(<BalanceCorrectionForm position={position} revision={1} onRecorded={recorded} />);
    const user = await review();
    expect(api.correctAssetBalance).not.toHaveBeenCalled();
    expect(screen.getByText('$60.00')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name:'Save correction'}));
    await waitFor(() => expect(recorded).toHaveBeenCalledOnce());
    expect(api.correctAssetBalance).toHaveBeenCalledWith(expect.objectContaining({account_kind:'court_credit',expected_cents:5300,target_cents:11300,reason:'Verified venue statement',request_id:expect.any(String)}));
    expect(screen.getByRole('status')).toHaveTextContent('Audit record saved');
  });

  it('reuses the same request ID when retrying an uncertain save', async () => {
    vi.mocked(api.correctAssetBalance).mockRejectedValueOnce(new Error('network'));
    render(<BalanceCorrectionForm position={position} revision={1} onRecorded={vi.fn()} />);
    const user = await review();
    await user.click(screen.getByRole('button', {name:'Save correction'}));
    await screen.findByRole('alert');
    const first = vi.mocked(api.correctAssetBalance).mock.calls[0][0];
    await user.click(screen.getByRole('button', {name:'Save correction'}));
    await waitFor(() => expect(api.correctAssetBalance).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.correctAssetBalance).mock.calls[1][0]).toEqual(first);
  });

  it('rejects sub-cent precision and unchanged balances', async () => {
    render(<BalanceCorrectionForm position={position} revision={1} onRecorded={vi.fn()} />);
    const user = await review('113.001');
    expect(screen.getByRole('alert')).toHaveTextContent('two decimal places');
    await user.clear(screen.getByLabelText('Correct balance (AUD)'));
    await user.type(screen.getByLabelText('Correct balance (AUD)'), '53');
    await user.click(screen.getByRole('button', {name:'Review correction'}));
    expect(screen.getByRole('alert')).toHaveTextContent('already the recorded balance');
    expect(api.correctAssetBalance).not.toHaveBeenCalled();
  });

  it('requires a fresh review after a stale balance rejection', async () => {
    const error = new AxiosError('changed');
    error.response = {data:{code:'balance_changed',message:'The balance changed.'}} as never;
    vi.mocked(api.correctAssetBalance).mockRejectedValueOnce(error);
    const recorded=vi.fn();
    render(<BalanceCorrectionForm position={position} revision={1} onRecorded={recorded} />);
    const user=await review();
    await user.click(screen.getByRole('button', {name:'Save correction'}));
    await waitFor(()=>expect(recorded).toHaveBeenCalledOnce());
    expect(screen.queryByRole('button',{name:'Save correction'})).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('balance changed');
  });
});
