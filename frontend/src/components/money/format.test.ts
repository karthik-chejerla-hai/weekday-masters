import { describe, it, expect } from 'vitest';
import { balanceState, formatCents, parseDollarsToCents } from './format';

describe('formatCents', () => {
  it('renders whole dollars and cents', () => {
    expect(formatCents(0)).toBe('$0.00');
    expect(formatCents(5)).toBe('$0.05');
    expect(formatCents(50)).toBe('$0.50');
    expect(formatCents(100)).toBe('$1.00');
    expect(formatCents(1694)).toBe('$16.94');
    expect(formatCents(14550)).toBe('$145.50');
  });

  it('groups thousands', () => {
    expect(formatCents(123456)).toBe('$1,234.56');
  });

  it('renders a debt as negative rather than hiding the sign', () => {
    expect(formatCents(-825)).toBe('-$8.25');
    expect(formatCents(-5)).toBe('-$0.05');
  });

  // The reason this function exists: cents are integers everywhere else, and
  // the awkward values are exactly the ones a float would get wrong.
  it('does not drift on values that float arithmetic rounds badly', () => {
    expect(formatCents(1)).toBe('$0.01');
    expect(formatCents(29)).toBe('$0.29');
    expect(formatCents(417)).toBe('$4.17');
    expect(formatCents(1010)).toBe('$10.10');
    expect(formatCents(2029)).toBe('$20.29');
  });
});

describe('parseDollarsToCents', () => {
  it('converts a typed amount to integer cents', () => {
    expect(parseDollarsToCents('50')).toBe(5000);
    expect(parseDollarsToCents('50.00')).toBe(5000);
    expect(parseDollarsToCents('16.94')).toBe(1694);
    expect(parseDollarsToCents('0.05')).toBe(5);
  });

  it('rounds rather than truncating a third decimal', () => {
    expect(parseDollarsToCents('4.166')).toBe(417);
    expect(parseDollarsToCents('4.164')).toBe(416);
  });

  it('reports nonsense as NaN so the form can refuse it', () => {
    expect(parseDollarsToCents('')).toBeNaN();
    expect(parseDollarsToCents('abc')).toBeNaN();
  });
});

describe('balanceState', () => {
  const threshold = 2000;

  it('flags a debt', () => {
    expect(balanceState(-1, threshold)).toBe('negative');
    expect(balanceState(-825, threshold)).toBe('negative');
  });

  it('flags a balance that will not cover the next session', () => {
    expect(balanceState(0, threshold)).toBe('low');
    expect(balanceState(1999, threshold)).toBe('low');
  });

  it('leaves a comfortable balance alone', () => {
    expect(balanceState(2000, threshold)).toBe('ok');
    expect(balanceState(4250, threshold)).toBe('ok');
  });
});
