import { test, expect, json } from './support/fixtures';

for (const identity of ['member', 'admin'] as const) {
  test.describe(`${identity} composer`, () => {
    test.use({ identity });
    test('examples stay editable until explicit Send and provider failures keep the text', async ({ page }) => {
      await page.route('**/api/assistant/status', route => json(route, { enabled: true }));
      let calls = 0;
      await page.route('**/api/assistant/messages', async route => {
        calls++;
        await json(route, { message: 'Assistant is temporarily unavailable. Try again.' }, 503);
      });
      await page.goto('/assistant');
      await expect(page.getByRole('heading', { level: 1, name: 'Ask Rally' })).toBeVisible();
      await page.getByRole('button', { name: 'Record a score', exact: true }).click();
      const input = page.getByLabel('Your message');
      await expect(input).toBeFocused();
      await expect(input).toHaveValue('Help me record a doubles game score.');
      expect(calls).toBe(0);
      await input.fill('Help me record the last game.');
      await page.getByRole('button', { name: 'Send message', exact: true }).click();
      await expect(page.getByRole('alert')).toContainText('temporarily unavailable');
      expect(calls).toBe(1);
      await expect(input).toHaveValue('Help me record the last game.');
      await expect(page.getByRole('link', { name: 'Choose a session to expense' })).toHaveCount(identity === 'admin' ? 1 : 0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    });
  });
}

test('recorder states stay separate from Send at 320px', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 844 });
  await page.addInitScript(() => {
    let grant!: (stream: unknown) => void;
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: {
      getUserMedia: () => new Promise(resolve => { grant = resolve; }),
    } });
    Object.defineProperty(window, 'grantTestMicrophone', { value: () => grant({ getTracks: () => [{ stop() {} }] }) });
    Object.defineProperty(window, 'MediaRecorder', { configurable: true, value: class {
      static isTypeSupported = () => true;
      state = 'inactive';
      mimeType = 'audio/webm';
      ondataavailable?: (event: { data: Blob }) => void;
      onstop?: () => void;
      start() { this.state = 'recording'; }
      stop() {
        this.state = 'inactive';
        this.ondataavailable?.({ data: new Blob(['synthetic recording'], { type: this.mimeType }) });
        this.onstop?.();
      }
    } });
  });
  await page.route('**/api/assistant/status', route => json(route, { enabled: true }));
  let finishTranscript!: () => void;
  await page.route('**/api/assistant/transcribe', async route => {
    await new Promise<void>(resolve => { finishTranscript = resolve; });
    await json(route, { text: 'Synthetic transcript for review' });
  });
  await page.goto('/assistant');
  const send = page.getByRole('button', { name: 'Send message', exact: true });
  async function expectSeparate(label: string) {
    const recorder = page.getByRole('button', { name: label, exact: true });
    await expect(recorder).toBeVisible();
    const a = (await recorder.boundingBox())!;
    const b = (await send.boundingBox())!;
    expect(a.x + a.width <= b.x || b.x + b.width <= a.x || a.y + a.height <= b.y || b.y + b.height <= a.y,
      `${label} must not overlap Send`).toBe(true);
    const form = (await page.locator('form').boundingBox())!;
    expect(a.x).toBeGreaterThanOrEqual(form.x);
    expect(a.x + a.width).toBeLessThanOrEqual(form.x + form.width);
  }
  await expectSeparate('Tap to speak');
  await page.getByRole('button', { name: 'Tap to speak' }).click();
  await expectSeparate('Waiting for microphone…');
  await page.evaluate(() => (window as unknown as { grantTestMicrophone: () => void }).grantTestMicrophone());
  await expectSeparate('Stop recording');
  await expectSeparate('Cancel recording');
  await page.getByRole('button', { name: 'Stop recording' }).click();
  await expectSeparate('Reading recording…');
  await expectSeparate('Cancel recording');
  await expect.poll(() => Boolean(finishTranscript)).toBe(true);
  finishTranscript();
  await expect(page.getByLabel('Your message')).toHaveValue('Synthetic transcript for review');
  await expectSeparate('Tap to speak');
});
