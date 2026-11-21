import { useCallback, useRef } from 'react';

/**
 * The `request_id` for a money form: one ID per submission, kept across retries.
 *
 * The server binds the ID to the contents it arrived with. If the reply is lost
 * after the write commits, pressing the button again sends the same ID and gets
 * the original transaction back instead of recording the receipt twice.
 *
 * `idFor(contents)` returns the same ID for as long as the contents are the
 * same, and mints a new one when they change: an edited form is a different
 * submission, and the server refuses a reused ID with different contents.
 * Call `reset()` once a submission has succeeded, so that recording the same
 * amount again is a second receipt and not a retry of the first.
 */
export function useSubmissionId() {
  const current = useRef<{ contents: string; id: string } | null>(null);

  const idFor = useCallback((contents: unknown): string => {
    const key = JSON.stringify(contents);
    if (current.current?.contents !== key) {
      current.current = { contents: key, id: crypto.randomUUID() };
    }
    return current.current.id;
  }, []);

  const reset = useCallback(() => {
    current.current = null;
  }, []);

  return { idFor, reset };
}
