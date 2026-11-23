import * as React from "react";
import { MotionConfig, motion } from "motion/react";

export function HeroTwigBadge() {
  return (
    <MotionConfig reducedMotion="user">
      <motion.span
        initial={{ scale: 0.85, rotate: -8 }}
        animate={{ scale: 1, rotate: 0 }}
        transition={{ type: "spring", stiffness: 340, damping: 20 }}
        whileHover={{ scale: 1.08, rotate: -6 }}
        whileTap={{ scale: 0.96 }}
        className="mx-1.5 inline-block size-11 cursor-pointer align-[-0.14em] rounded-2xl shadow-[0_12px_36px_rgba(88,204,125,0.35)] sm:mx-2.5 sm:size-16 xl:size-[4.25rem]"
      >
        <img
          src="/favicon.svg"
          alt=""
          width={68}
          height={68}
          className="block size-full rounded-2xl"
        />
      </motion.span>
    </MotionConfig>
  );
}
