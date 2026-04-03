/**
 * FloqIcon — the Floq brand icon.
 *
 * variant="circle" (default): blue circle with white star + plus accent
 * variant="flat": just the star shape in currentColor (no circle bg)
 */
export function FloqIcon({
  size = 40,
  variant = "circle",
  className,
}: {
  size?: number;
  variant?: "circle" | "flat";
  className?: string;
}) {
  if (variant === "flat") {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 32 32"
        fill="currentColor"
        className={className}
      >
        {/* Four-pointed star */}
        <path d="M15 2C15 2 17 9 19 12.5C21 16 28 17 28 17C28 17 21 18 19 21.5C17 25 15 30 15 30C15 30 13 25 11 21.5C9 18 2 17 2 17C2 17 9 16 11 12.5C13 9 15 2 15 2Z" />
        {/* Plus accent */}
        <line x1="25" y1="4" x2="25" y2="10" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
        <line x1="22" y1="7" x2="28" y2="7" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
      </svg>
    );
  }

  // Circle variant (default)
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      fill="none"
      className={className}
    >
      <circle cx="24" cy="24" r="24" fill="#004ac6" />
      <path
        d="M24 10C24 10 26 18 28 21C30 24 36 25 36 25C36 25 30 26 28 29C26 32 24 38 24 38C24 38 22 32 20 29C18 26 12 25 12 25C12 25 18 24 20 21C22 18 24 10 24 10Z"
        fill="white"
      />
      <circle cx="34" cy="14" r="4.5" fill="#004ac6" />
      <circle cx="34" cy="14" r="3.5" fill="white" opacity="0.9" />
      <line x1="34" y1="11.5" x2="34" y2="16.5" stroke="#004ac6" strokeWidth="1.5" strokeLinecap="round" />
      <line x1="31.5" y1="14" x2="36.5" y2="14" stroke="#004ac6" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
