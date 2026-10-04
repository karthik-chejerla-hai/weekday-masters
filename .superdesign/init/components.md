# Shared UI Components

Framework: React 18 + TypeScript. Styling: Tailwind CSS with a small set of global component classes.

## Avatar

Path: `frontend/src/components/ui/Avatar.tsx`

```tsx
import { User } from 'lucide-react';

interface AvatarProps {
  src?: string;
  name: string;
  size?: 'sm' | 'md' | 'lg';
}

const sizeClasses = {
  sm: 'w-8 h-8 text-xs',
  md: 'w-10 h-10 text-sm',
  lg: 'w-16 h-16 text-xl',
};

export default function Avatar({ src, name, size = 'md' }: AvatarProps) {
  const initials = name.split(' ').map((n) => n[0]).join('').toUpperCase().slice(0, 2);
  if (src) {
    const imageSizes = { sm: 64, md: 80, lg: 128 };
    const optimizedSrc = src.includes('googleusercontent.com')
      ? `${src.split('=')[0]}=s${imageSizes[size]}`
      : src;
    return <img src={optimizedSrc} alt={name} className={`${sizeClasses[size]} rounded-full object-cover`} />;
  }
  return (
    <div className={`${sizeClasses[size]} rounded-full bg-primary-100 text-primary-700 flex items-center justify-center font-medium`}>
      {initials || <User className="w-1/2 h-1/2" />}
    </div>
  );
}
```

## Badge

Path: `frontend/src/components/ui/Badge.tsx`

```tsx
import { ReactNode } from 'react';

interface BadgeProps {
  children: ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info';
}

const variantClasses = {
  default: 'bg-slate-100 text-slate-700',
  success: 'bg-green-100 text-green-700',
  warning: 'bg-amber-100 text-amber-700',
  danger: 'bg-red-100 text-red-700',
  info: 'bg-blue-100 text-blue-700',
};

export default function Badge({ children, variant = 'default' }: BadgeProps) {
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${variantClasses[variant]}`}>
      {children}
    </span>
  );
}
```

## Loading

Path: `frontend/src/components/ui/Loading.tsx`

```tsx
import { Loader2 } from 'lucide-react';

export default function Loading() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-50">
      <div className="text-center">
        <Loader2 className="w-12 h-12 text-primary-600 animate-spin mx-auto" />
        <p className="mt-4 text-slate-600">Loading...</p>
      </div>
    </div>
  );
}
```

## BalanceChip

Path: `frontend/src/components/money/BalanceChip.tsx`

```tsx
import type { BalanceState } from '../../types';
import { formatCents } from './format';

const TONE: Record<BalanceState, string> = {
  ok: 'bg-primary-50 text-primary-700 border-primary-200',
  low: 'bg-secondary-50 text-secondary-700 border-secondary-300',
  negative: 'bg-red-50 text-red-700 border-red-200',
};

interface BalanceChipProps {
  cents: number;
  state: BalanceState;
  compact?: boolean;
  title?: string;
}

export default function BalanceChip({ cents, state, compact = false, title }: BalanceChipProps) {
  return (
    <span className={`inline-flex items-center rounded-full border font-medium tabular-nums ${TONE[state]} ${compact ? 'px-2 py-0.5 text-xs' : 'px-3 py-1 text-sm'}`} title={title ?? (state === 'negative' ? 'You owe the club' : undefined)}>
      {formatCents(cents)}
    </span>
  );
}
```

