import * as React from "react";
import { MotionConfig, motion, useReducedMotion } from "motion/react";

const INK = "#111312";
const EASE = [0.22, 1, 0.36, 1] as const;

/**
 * The memtwig mark drawn inline: the stem grows, two branches split off (one per vault),
 * then the bud on the active branch lands. On hover a signal runs up the stem into that branch,
 * the same path a recalled note takes from the vault to the agent.
 */
export function HeroTwigBadge() {
  const prefersReduced = useReducedMotion();
  const [signal, setSignal] = React.useState(0);
  const fire = () => {
    if (!prefersReduced) setSignal((n) => n + 1);
  };

  return (
    <MotionConfig reducedMotion="user">
      <motion.span
        whileHover={{ scale: 1.05, rotate: -4 }}
        whileTap={{ scale: 0.97 }}
        transition={{ type: "spring", stiffness: 340, damping: 20 }}
        onHoverStart={fire}
        onTap={fire}
        className="twig-badge mx-1.5 inline-block size-11 cursor-pointer align-[-0.14em] rounded-2xl shadow-[0_12px_36px_rgba(88,204,125,0.32)] sm:mx-2.5 sm:size-16 xl:size-[4.25rem]"
        aria-hidden="true"
        tabIndex={-1}
      >
        <svg viewBox="0 0 32 32" className="block size-full" fill="none">
          <rect width="32" height="32" rx="8" fill="#58cc7d" />
          {/* stem, then the active branch, then the second vault's branch */}
          <path className="twig-stroke" pathLength={1} d="M16 24.5V8" stroke={INK} strokeWidth="2.6" strokeLinecap="round" style={{ ["--d" as string]: "0.15s" }} />
          <path className="twig-stroke" pathLength={1} d="M16 18.2L22 12.2" stroke={INK} strokeWidth="2.6" strokeLinecap="round" style={{ ["--d" as string]: "0.55s" }} />
          <path className="twig-stroke" pathLength={1} d="M16 13.8L10.2 9.4" stroke={INK} strokeWidth="2.6" strokeLinecap="round" style={{ ["--d" as string]: "0.7s" }} />
          {/* bud on the active branch */}
          <circle className="twig-bud" cx="22.5" cy="11.7" r="2.2" fill={INK} />
          {/* recall signal: runs up the stem into the active branch */}
          {signal > 0 && (
            <motion.path
              key={signal}
              d="M16 24.5V18.2L22 12.2"
              pathLength={1}
              stroke="#effcf3"
              strokeWidth="1.3"
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeDasharray="0.28 1.4"
              initial={{ strokeDashoffset: 0.28, opacity: 1 }}
              animate={{ strokeDashoffset: -1, opacity: [1, 1, 0] }}
              transition={{ duration: 0.75, ease: EASE, times: [0, 0.8, 1] }}
            />
          )}
        </svg>
      </motion.span>
    </MotionConfig>
  );
}
