import { FC } from 'react';

interface AvariLogoProps {
  className?: string;
  size?: number;
}

export const AvariLogo: FC<AvariLogoProps> = ({ className = 'w-7 h-7', size }) => {
  return (
    <img
      src="/assets/logo_star.png"
      alt="Avari Logo"
      className={className}
      style={size ? { width: size, height: size } : undefined}
      width={size || 28}
      height={size || 28}
    />
  );
};
