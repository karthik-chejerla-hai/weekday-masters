export function EnvironmentBanner() {
  if (import.meta.env.VITE_APP_ENVIRONMENT !== 'dev') return null;
  return (
    <div role="status" className="sticky top-0 z-[100] bg-amber-300 px-3 py-1 text-center text-sm font-bold text-slate-900">
      DEV · Test data · {import.meta.env.VITE_RELEASE_SHA?.slice(0, 7)}
    </div>
  );
}
