import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AxiosError } from 'axios';
import { api } from '../../services/api';
import AssetPurchaseHistory, { PurchaseCorrectionForm } from './AssetPurchaseHistory';
import type { AssetPurchase } from '../../types';

vi.mock('../../services/api', () => ({ api: { listAssetPurchases: vi.fn(), correctAssetPurchase: vi.fn() } }));
const purchase: AssetPurchase = { id:'purchase-1', actor_name:'Alex', kind:'shuttle_purchase', amount_cents:5000, units:12, reversed:false, description:'Shuttle receipt', occurred_at:'2026-10-01T06:00:00Z', created_at:'2026-10-01T06:00:00Z', created_by:'admin' };
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listAssetPurchases).mockResolvedValue({ items:[purchase], total:1 });
  vi.mocked(api.correctAssetPurchase).mockResolvedValue({} as never);
});

async function prepare(amount='60.00') {
  const user = userEvent.setup();
  await user.clear(screen.getByLabelText('Correct purchase amount (AUD)'));
  await user.type(screen.getByLabelText('Correct purchase amount (AUD)'), amount);
  await user.clear(screen.getByLabelText('Correct shuttle count'));
  await user.type(screen.getByLabelText('Correct shuttle count'), '24');
  await user.type(screen.getByLabelText('Reason'), 'Receipt verified');
  await user.click(screen.getByRole('button', { name:'Review correction' }));
  return user;
}

describe('asset purchase corrections', () => {
  it('reviews purchase, bank and stock changes before posting', async () => {
    const recorded=vi.fn();
    render(<PurchaseCorrectionForm purchase={purchase} onRecorded={recorded} onCancel={vi.fn()} />);
    const user=await prepare();
    expect(api.correctAssetPurchase).not.toHaveBeenCalled();
    expect(screen.getByText('Purchase: $50.00 → $60.00')).toBeInTheDocument();
    expect(screen.getByText('Shuttles: 12 → 24')).toBeInTheDocument();
    expect(screen.getByText('Bank adjustment: -$10.00')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name:'Save purchase correction' }));
    await waitFor(() => expect(recorded).toHaveBeenCalledOnce());
    expect(api.correctAssetPurchase).toHaveBeenCalledWith(purchase.id, { amount_cents:6000, units:24, reason:'Receipt verified' });
  });

  it('keeps the reviewed correction for safe retry and displays stock rejection', async () => {
    const err=new AxiosError('stock');
    err.response={ data:{ message:'This correction would leave invalid shuttle stock.' } } as never;
    vi.mocked(api.correctAssetPurchase).mockRejectedValueOnce(err);
    render(<PurchaseCorrectionForm purchase={purchase} onRecorded={vi.fn()} onCancel={vi.fn()} />);
    const user=await prepare();
    await user.click(screen.getByRole('button', { name:'Save purchase correction' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid shuttle stock');
    const first=vi.mocked(api.correctAssetPurchase).mock.calls[0];
    await user.click(screen.getByRole('button', { name:'Save purchase correction' }));
    await waitFor(() => expect(api.correctAssetPurchase).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.correctAssetPurchase).mock.calls[1]).toEqual(first);
  });

  it('rejects sub-cent inputs', async () => {
    render(<PurchaseCorrectionForm purchase={purchase} onRecorded={vi.fn()} onCancel={vi.fn()} />);
    await prepare('60.001');
    expect(screen.getByRole('alert')).toHaveTextContent('two decimal places');
    expect(api.correctAssetPurchase).not.toHaveBeenCalled();
  });

  it('corrects court purchases without a shuttle count', async () => {
    render(<PurchaseCorrectionForm purchase={{...purchase, kind:'court_credit_purchase', units:0}} onRecorded={vi.fn()} onCancel={vi.fn()} />);
    const user=userEvent.setup();
    expect(screen.queryByLabelText('Correct shuttle count')).not.toBeInTheDocument();
    await user.clear(screen.getByLabelText('Correct purchase amount (AUD)'));
    await user.type(screen.getByLabelText('Correct purchase amount (AUD)'), '100');
    await user.type(screen.getByLabelText('Reason'), 'Venue receipt');
    await user.click(screen.getByRole('button', {name:'Review correction'}));
    expect(screen.getByText('Court credit adjustment: $50.00')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name:'Save purchase correction'}));
    await waitFor(() => expect(api.correctAssetPurchase).toHaveBeenCalledWith(purchase.id, {amount_cents:10000,units:0,reason:'Venue receipt'}));
  });

  it('loads history on opening and disables correction of reversed purchases', async () => {
    vi.mocked(api.listAssetPurchases).mockResolvedValue({items:[{...purchase,reversed:true}],total:1});
    render(<AssetPurchaseHistory revision={1} onRecorded={vi.fn()} />);
    expect(api.listAssetPurchases).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText('Purchase history and corrections'));
    expect(await screen.findByText('Reversed / corrected')).toBeInTheDocument();
    expect(screen.queryByRole('button',{name:'Correct purchase'})).not.toBeInTheDocument();
  });
});
