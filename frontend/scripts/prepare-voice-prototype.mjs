// Throwaway voice study assets. No remote service or runtime CDN is used.
import { mkdir, readdir, copyFile, rm } from 'node:fs/promises';
const target = new URL('../public/prototype-voice-assets/', import.meta.url);
await rm(target, { recursive: true, force: true });
await mkdir(target, { recursive: true });
for (const packagePath of ['@ricky0123/vad-web', 'onnxruntime-web']) {
  const source = new URL(`../node_modules/${packagePath}/dist/`, import.meta.url);
  for (const name of await readdir(source)) {
    if (['silero_vad_v5.onnx', 'vad.worklet.bundle.min.js', 'ort-wasm-simd-threaded.mjs', 'ort-wasm-simd-threaded.wasm'].includes(name)) {
      await copyFile(new URL(name, source), new URL(name, target));
    }
  }
}
