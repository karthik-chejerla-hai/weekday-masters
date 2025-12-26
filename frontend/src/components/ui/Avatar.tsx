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
  const initials = name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);

  if (src) {
    // Add size parameter for Google profile pictures to reduce rate limiting
    const imageSizes = { sm: 64, md: 80, lg: 128 };
    const optimizedSrc = src.includes('googleusercontent.com')
      ? `${src.split('=')[0]}=s${imageSizes[size]}`
      : src;

    return (
      <img
        src={optimizedSrc}
        alt={name}
        className={`${sizeClasses[size]} rounded-full object-cover`}
      />
    );
  }

  return (
    <div
      className={`${sizeClasses[size]} rounded-full bg-primary-100 text-primary-700 flex items-center justify-center font-medium`}
    >
      {initials || <User className="w-1/2 h-1/2" />}
    </div>
  );
}
