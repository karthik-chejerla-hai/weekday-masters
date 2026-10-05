import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import VoiceRecorder from './VoiceRecorder';
import { api } from '../../services/api';
vi.mock('../../services/api', () => ({ api: { transcribeAudio: vi.fn() } }));
const stopTrack = vi.fn();
class Recorder {
  static instances: Recorder[] = [];
  static isTypeSupported = (type: string) => type === 'audio/mp4';
  state = 'inactive'; mimeType = 'audio/mp4';
  ondataavailable?: (event: { data: Blob }) => void;
  onstop?: () => void;
  onerror?: () => void;
  constructor() { Recorder.instances.push(this); }
  start() { this.state = 'recording'; }
  stop() { this.state = 'inactive'; this.ondataavailable?.({ data: new Blob(['audio'], { type: this.mimeType }) }); this.onstop?.(); }
}
beforeEach(() => {
  vi.clearAllMocks(); Recorder.instances = [];
  vi.stubGlobal('MediaRecorder', Recorder);
  Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: { getUserMedia: vi.fn().mockResolvedValue({ getTracks: () => [{ stop: stopTrack }] }) } });
  vi.mocked(api.transcribeAudio).mockResolvedValue({ text: 'Eight shuttles' });
});
afterEach(() => vi.unstubAllGlobals());
it('uploads the actual recording format and releases the microphone', async () => {
  const user = userEvent.setup(); const transcript = vi.fn(); render(<VoiceRecorder onTranscript={transcript} />);
  await user.click(screen.getByRole('button', { name: 'Tap to speak' }));
  await user.click(await screen.findByRole('button', { name: 'Stop recording' }));
  await waitFor(() => expect(transcript).toHaveBeenCalledWith('Eight shuttles'));
  expect(vi.mocked(api.transcribeAudio).mock.calls[0][0].type).toBe('audio/mp4');
  expect(stopTrack).toHaveBeenCalled();
});
it('does not upload a cancelled recording', async () => {
  const user = userEvent.setup(); render(<VoiceRecorder onTranscript={vi.fn()} />);
  await user.click(screen.getByRole('button', { name: 'Tap to speak' }));
  await user.click(await screen.findByRole('button', { name: 'Cancel recording' }));
  expect(api.transcribeAudio).not.toHaveBeenCalled(); expect(stopTrack).toHaveBeenCalled();
});
it('releases tracks on unmount without uploading', async () => {
  const user = userEvent.setup(); const view = render(<VoiceRecorder onTranscript={vi.fn()} />);
  await user.click(screen.getByRole('button', { name: 'Tap to speak' }));
  await screen.findByRole('button', { name: 'Stop recording' }); view.unmount();
  expect(stopTrack).toHaveBeenCalled(); expect(api.transcribeAudio).not.toHaveBeenCalled();
});
it('handles microphone denial', async () => {
  vi.mocked(navigator.mediaDevices.getUserMedia).mockRejectedValue({ name: 'NotAllowedError' });
  const user = userEvent.setup(); render(<VoiceRecorder onTranscript={vi.fn()} />);
  await user.click(screen.getByRole('button', { name: 'Tap to speak' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('Microphone access was denied');
});
it('releases a stream granted after cancellation', async () => {
  let resolve!: (stream: MediaStream) => void;
  vi.mocked(navigator.mediaDevices.getUserMedia).mockReturnValue(new Promise((done) => { resolve = done; }));
  const user = userEvent.setup(); render(<VoiceRecorder onTranscript={vi.fn()} />);
  await user.click(screen.getByRole('button', { name: 'Tap to speak' }));
  await user.click(screen.getByRole('button', { name: 'Cancel recording' }));
  await act(async () => resolve({ getTracks: () => [{ stop: stopTrack }] } as unknown as MediaStream));
  expect(stopTrack).toHaveBeenCalled(); expect(Recorder.instances).toHaveLength(0);
});

it('keeps the new recording time limit when an old stop event arrives late', async () => {
  vi.useFakeTimers();
  const view = render(<VoiceRecorder onTranscript={vi.fn()} />);
  try {
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Tap to speak' })); });
    const old = Recorder.instances[0];
    vi.spyOn(old, 'stop').mockImplementation(() => { old.state = 'inactive'; });
    fireEvent.click(screen.getByRole('button', { name: 'Cancel recording' }));
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Tap to speak' })); });
    await act(async () => { old.onstop?.(); });
    expect(Recorder.instances[1].state).toBe('recording');
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
    expect(Recorder.instances[1].state).toBe('inactive');
    expect(api.transcribeAudio).toHaveBeenCalledOnce();
  } finally {
    view.unmount();
    vi.useRealTimers();
  }
});
