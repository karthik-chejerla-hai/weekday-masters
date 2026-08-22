import { useState } from 'react';
import { Check, X, HelpCircle, Loader2, Hourglass } from 'lucide-react';
import type { RSVPStatus, SelectableRSVPStatus } from '../../types';

interface RSVPButtonProps {
  currentStatus?: RSVPStatus;
  onRSVP: (status: SelectableRSVPStatus) => Promise<void>;
  disabled?: boolean;
  /** Session is at capacity, so picking "in" will join the waitlist instead. */
  isFull?: boolean;
  /** 1-based queue position when the current user is waitlisted. */
  waitlistPosition?: number;
}

export default function RSVPButton({
  currentStatus,
  onRSVP,
  disabled,
  isFull,
  waitlistPosition,
}: RSVPButtonProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [loadingStatus, setLoadingStatus] = useState<SelectableRSVPStatus | null>(null);

  const isWaitlisted = currentStatus === 'waitlisted';

  const handleClick = async (status: SelectableRSVPStatus) => {
    if (isLoading || disabled) return;

    setIsLoading(true);
    setLoadingStatus(status);
    try {
      await onRSVP(status);
    } finally {
      setIsLoading(false);
      setLoadingStatus(null);
    }
  };

  // The "in" button doubles as the waitlist control: when the session is full it
  // joins the queue, and when already queued it reflects the position.
  const queueing = isWaitlisted || isFull;
  const inLabel = isWaitlisted
    ? waitlistPosition
      ? `Waitlisted #${waitlistPosition}`
      : 'Waitlisted'
    : isFull
      ? 'Join Waitlist'
      : "I'm In";

  const buttons: {
    status: SelectableRSVPStatus;
    icon: typeof Check;
    label: string;
    activeClass: string;
  }[] = [
    {
      status: 'in',
      icon: queueing ? Hourglass : Check,
      label: inLabel,
      activeClass: isWaitlisted
        ? 'bg-amber-500 text-white border-amber-500'
        : 'bg-green-600 text-white border-green-600',
    },
    {
      status: 'maybe',
      icon: HelpCircle,
      label: 'Maybe',
      activeClass: 'bg-amber-500 text-white border-amber-500',
    },
    {
      status: 'out',
      icon: X,
      label: "Can't Make It",
      activeClass: 'bg-red-600 text-white border-red-600',
    },
  ];

  return (
    <div className="flex gap-2">
      {buttons.map(({ status, icon: Icon, label, activeClass }) => {
        // A waitlisted player is queued for the "in" spot, so that button reads active.
        const isActive = currentStatus === status || (status === 'in' && isWaitlisted);
        const isLoadingThis = loadingStatus === status;

        return (
          <button
            key={status}
            onClick={() => handleClick(status)}
            disabled={disabled || isLoading}
            className={`
              flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-lg border-2 font-medium transition-all
              ${isActive
                ? activeClass
                : 'border-slate-300 text-slate-600 hover:border-slate-400 hover:bg-slate-50'
              }
              ${disabled ? 'opacity-50 cursor-not-allowed' : ''}
            `}
          >
            {isLoadingThis ? (
              <Loader2 className="w-5 h-5 animate-spin" />
            ) : (
              <Icon className="w-5 h-5" />
            )}
            <span className="hidden sm:inline">{label}</span>
          </button>
        );
      })}
    </div>
  );
}
