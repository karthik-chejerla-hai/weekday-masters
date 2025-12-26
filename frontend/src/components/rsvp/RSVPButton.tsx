import { useState } from 'react';
import { Check, X, HelpCircle, Loader2 } from 'lucide-react';
import type { RSVPStatus } from '../../types';

interface RSVPButtonProps {
  currentStatus?: RSVPStatus;
  onRSVP: (status: RSVPStatus) => Promise<void>;
  disabled?: boolean;
}

export default function RSVPButton({ currentStatus, onRSVP, disabled }: RSVPButtonProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [loadingStatus, setLoadingStatus] = useState<RSVPStatus | null>(null);

  const handleClick = async (status: RSVPStatus) => {
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

  const buttons: { status: RSVPStatus; icon: typeof Check; label: string; activeClass: string }[] = [
    { status: 'in', icon: Check, label: "I'm In", activeClass: 'bg-green-600 text-white border-green-600' },
    { status: 'maybe', icon: HelpCircle, label: 'Maybe', activeClass: 'bg-amber-500 text-white border-amber-500' },
    { status: 'out', icon: X, label: "Can't Make It", activeClass: 'bg-red-600 text-white border-red-600' },
  ];

  return (
    <div className="flex gap-2">
      {buttons.map(({ status, icon: Icon, label, activeClass }) => {
        const isActive = currentStatus === status;
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
