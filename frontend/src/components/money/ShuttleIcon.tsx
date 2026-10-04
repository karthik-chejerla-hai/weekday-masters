import type { SVGProps } from 'react';

export default function ShuttleIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 32 32" fill="currentColor" aria-hidden="true" {...props}>
      <g transform="rotate(45 16 16)">
        <path d="M11.5 8a4.5 4.5 0 0 1 9 0v1.5h-9Z" />
        <rect x="11" y="10.5" width="10" height="2" rx="0.6" />
        <path
          d="m12 12-4.5 11m6.5-11-2.5 12M16 12v13m2-13 2.5 12M20 12l4.5 11M10.5 16h11M9.3 19h13.4"
          fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round"
        />
        <path d="m9.6 19-4.1 5.6q-.5.8.6 1.3l1.8.5 3.6-6.7Zm2.8.7-3 6.8q.2.9 2.3 1l2.4-7.3Zm2.1.5-.9 6.9q-.1 1.2 2.4 1.5 2.5-.3 2.4-1.5l-.9-6.9Zm3.4 0 2.4 7.3q2.1-.1 2.3-1l-3-6.8Zm2.6-.5 3.6 6.7 1.8-.5q1.1-.5.6-1.3L22.4 19Z" />
      </g>
    </svg>
  );
}
