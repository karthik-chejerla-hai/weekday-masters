import { test, expect, json, detail, members, NOW } from './support/fixtures';
import type { GameInput, GameResult } from '../src/types';

test('score review validates players and scores, retries safely and persists the result', async ({ page }) => {
  await page.route('**/api/sessions/session-1', route => json(route, detail({
    starts_at: '2026-10-06T07:00:00Z', ends_at: '2026-10-06T09:00:00Z', session_date: '2026-10-06',
  })));
  const results: GameResult[] = [];
  const writes: GameInput[] = [];
  let fail = true;
  await page.route('**/api/sessions/session-1/games?*', route => json(route, { items: results, total: results.length }));
  await page.route('**/api/sessions/session-1/games', route => {
    expect(route.request().method()).toBe('POST');
    const input: GameInput = route.request().postDataJSON();
    writes.push(input);
    if (fail) return json(route, { error: 'Score service unavailable' }, 503);
    const players = (ids: string[]) => ids.map(id => ({ id, name: members.find(m => m.id === id)!.nickname }));
    const game: GameResult = { id: 'game-1', session_id: 'session-1', session_title: 'Friday badminton',
      session_date: '2026-10-06', team_a: players(input.team_a), team_b: players(input.team_b),
      score_a: input.score_a, score_b: input.score_b, version: 1, created_by: members[0].id,
      updated_by: members[0].id, recorder_name: 'Alex', editor_name: 'Alex', created_at: NOW,
      updated_at: NOW, voided_at: null };
    results.push(game);
    return json(route, game);
  });
  await page.goto('/sessions/session-1/games');
  await page.getByText('Enter a score by hand', { exact: true }).click();
  const form = page.getByRole('form', { name: 'Game result' });
  await form.getByRole('button', { name: 'Save game' }).click();
  await expect(form.getByRole('alert')).toHaveText('Choose four different players.');
  for (const [index, label] of ['Team A player 1', 'Team A player 2', 'Team B player 1', 'Team B player 2'].entries()) {
    await form.getByLabel(label).selectOption(members[index].id);
  }
  await form.getByLabel('Team A score').fill('21');
  await form.getByLabel('Team B score').fill('21');
  await form.getByRole('button', { name: 'Save game' }).click();
  await expect(form.getByRole('alert')).toHaveText('Enter two different whole-number scores from 0 to 99.');
  expect(writes).toHaveLength(0);
  await form.getByLabel('Team B score').fill('18');
  await form.getByRole('button', { name: 'Save game' }).click();
  await expect(form.getByRole('alert')).toBeVisible();
  await expect(form.getByLabel('Team B score')).toHaveValue('18');
  fail = false;
  await form.getByRole('button', { name: 'Save game' }).click();
  const history = page.getByRole('region', { name: 'Session game history' });
  await expect(history.getByRole('heading', { name: 'Recorded games (1)' })).toBeVisible();
  await expect(history).toContainText('21 : 18');
  expect(writes).toHaveLength(2);
  expect(writes[0]).toEqual(writes[1]);
  expect(writes[1]).toMatchObject({ team_a: ['member-1', 'member-2'], team_b: ['member-3', 'member-4'], score_a: 21, score_b: 18, request_id: expect.any(String) });
  await page.reload();
  await expect(history).toContainText('Alex + Blair');
  await expect(history).toContainText('Casey + Drew');
  await expect(history).toContainText('21 : 18');
});

test('future session cannot accept game scores', async ({ page }) => {
  await page.goto('/sessions/session-1/games');
  await expect(page.getByText('Score entry opens when this session starts.')).toBeVisible();
  await expect(page.getByRole('form', { name: 'Game result' })).toHaveCount(0);
});
